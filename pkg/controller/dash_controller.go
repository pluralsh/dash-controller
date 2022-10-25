package controller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	dashv1alpha1 "github.com/pluralsh/dash-controller/apis/dash/v1alpha1"
	"github.com/pluralsh/dash-controller/pkg/kubernetes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	IngressFinalizer    = "pluralsh.dash-controller/ingress-protection"
	DeploymentFinalizer = "pluralsh.dash-controller/deployment-protection"
	ServiceFinalizer    = "pluralsh.dash-controller/service-protection"
)

// Reconciler reconciles a DatabaseRequest object
type Reconciler struct {
	client.Client
	Log logr.Logger
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("Dash", req.NamespacedName)

	dashApp := &dashv1alpha1.DashApplication{}
	if err := r.Get(ctx, req.NamespacedName, dashApp); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	name := dashApp.Name
	namespace := dashApp.Namespace

	if !dashApp.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(dashApp, IngressFinalizer) {
			log.Info("delete ingress")
			if err := r.Delete(ctx, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
			kubernetes.TryRemoveFinalizer(ctx, r.Client, dashApp, IngressFinalizer)
		}
		if controllerutil.ContainsFinalizer(dashApp, ServiceFinalizer) {
			log.Info("delete service")
			if err := r.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
			kubernetes.TryRemoveFinalizer(ctx, r.Client, dashApp, ServiceFinalizer)
		}
		if controllerutil.ContainsFinalizer(dashApp, DeploymentFinalizer) {
			log.Info("delete deployment")
			if err := r.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
			kubernetes.TryRemoveFinalizer(ctx, r.Client, dashApp, DeploymentFinalizer)
		}
		return ctrl.Result{}, nil
	}

	if err := r.createUpdateDeployment(ctx, log, dashApp); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.createUpdateService(ctx, log, dashApp); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.createUpdateIngress(ctx, log, dashApp); err != nil {
		return ctrl.Result{}, err
	}

	dashApp.Status.Ready = true
	if err := r.Status().Update(ctx, dashApp); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// Generate the desired Service object for the workspace
func generateService(dashApp *dashv1alpha1.DashApplication) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dashApp.Name,
			Namespace:   dashApp.Namespace,
			Labels:      baseAppLabels(dashApp.Name, nil),
			Annotations: dashApp.Spec.ServiceAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Selector: baseAppLabels(dashApp.Name, nil),
			Ports: []corev1.ServicePort{{
				Protocol:   corev1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.FromString(dashApp.Name),
			}},
		},
	}

	if dashApp.Spec.Ingress == nil {
		svc.Spec.Type = "LoadBalancer"
	}

	return svc
}

func genIngress(dashApp *dashv1alpha1.DashApplication) *networkingv1.Ingress {
	prefix := networkingv1.PathTypePrefix
	path := "/"
	if dashApp.Spec.Ingress.Path != "" {
		path = dashApp.Spec.Ingress.Path
	}
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dashApp.Name,
			Namespace:   dashApp.Namespace,
			Labels:      baseAppLabels(dashApp.Name, nil),
			Annotations: dashApp.Spec.Ingress.Annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: dashApp.Spec.Ingress.IngressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: dashApp.Spec.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: &prefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: dashApp.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return ingress
}

func genDeployment(dashApp *dashv1alpha1.DashApplication) *appsv1.Deployment {
	name := dashApp.Name
	var envVars []corev1.EnvVar

	if dashApp.Spec.Ingress != nil && dashApp.Spec.Ingress.Path != "" && dashApp.Spec.Ingress.Path != "/" {
		envVars = []corev1.EnvVar{
			{
				Name:  "DASH_ROUTES_PATHNAME_PREFIX",
				Value: fmt.Sprintf("%s/", dashApp.Spec.Ingress.Path),
			},
		}
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: dashApp.Namespace,
			Labels:    baseAppLabels(name, dashApp.Spec.Labels),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: dashApp.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: baseAppLabels(name, nil),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: baseAppLabels(name, dashApp.Spec.Labels),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            name,
							ImagePullPolicy: corev1.PullAlways,
							Image:           dashApp.Spec.Container.Image,
							Args:            dashApp.Spec.Container.Args,
							Command:         dashApp.Spec.Container.Command,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: dashApp.Spec.Container.ContainerPort,
									Protocol:      corev1.ProtocolTCP,
									Name:          name,
								},
							},
							Env: envVars,
						},
					},
				},
			},
		},
	}

	return deployment
}

func baseAppLabels(name string, additionalLabels map[string]string) map[string]string {
	labels := map[string]string{
		"dash.plural.sh/name": name,
	}
	for k, v := range additionalLabels {
		labels[k] = v
	}
	return labels
}

func (r *Reconciler) createUpdateIngress(ctx context.Context, log logr.Logger, dashApp *dashv1alpha1.DashApplication) error {
	if dashApp.Spec.Ingress != nil {
		update := false
		name := dashApp.Name
		namespace := dashApp.Namespace
		newIngress := genIngress(dashApp)
		ingress := &networkingv1.Ingress{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, ingress); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}

			log.Info("create ingress")
			if err := r.Create(ctx, newIngress); err != nil {
				return err
			}
			if err := kubernetes.TryAddFinalizer(ctx, r.Client, dashApp, IngressFinalizer); err != nil {
				return err
			}
			return nil
		}
		if !reflect.DeepEqual(ingress.Annotations, newIngress.Annotations) {
			update = true
			ingress.Annotations = newIngress.Annotations
		}
		if !reflect.DeepEqual(ingress.Spec.IngressClassName, newIngress.Spec.IngressClassName) {
			update = true
			ingress.Spec.IngressClassName = newIngress.Spec.IngressClassName
		}
		if !reflect.DeepEqual(ingress.Spec.Rules[0].Host, newIngress.Spec.Rules[0].Host) {
			update = true
			ingress.Spec.Rules[0].Host = newIngress.Spec.Rules[0].Host
		}
		if !reflect.DeepEqual(ingress.Spec.Rules[0].HTTP.Paths, newIngress.Spec.Rules[0].HTTP.Paths) {
			update = true
			ingress.Spec.Rules[0].HTTP.Paths = newIngress.Spec.Rules[0].HTTP.Paths
		}

		if update {
			log.Info("update ingress")
			return r.Update(ctx, ingress)
		}
	}
	return nil
}

func (r *Reconciler) createUpdateService(ctx context.Context, log logr.Logger, dashApp *dashv1alpha1.DashApplication) error {
	name := dashApp.Name
	namespace := dashApp.Namespace
	newService := generateService(dashApp)
	svc := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, svc); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("create service")
		if err := r.Create(ctx, newService); err != nil {
			return err
		}
		if err := kubernetes.TryAddFinalizer(ctx, r.Client, dashApp, ServiceFinalizer); err != nil {
			return err
		}
		return nil
	}

	if !reflect.DeepEqual(newService.Annotations, svc.Annotations) {
		svc.Annotations = newService.Annotations
		log.Info("update service")
		return r.Update(ctx, svc)
	}

	return nil
}

func (r *Reconciler) createUpdateDeployment(ctx context.Context, log logr.Logger, dashApp *dashv1alpha1.DashApplication) error {
	var update bool
	name := dashApp.Name
	namespace := dashApp.Namespace
	newDeployment := genDeployment(dashApp)
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("create deployment")
		if err := r.Create(ctx, newDeployment); err != nil {
			return err
		}
		if err := kubernetes.TryAddFinalizer(ctx, r.Client, dashApp, DeploymentFinalizer); err != nil {
			return err
		}
		return nil
	}

	if !reflect.DeepEqual(newDeployment.Spec.Replicas, deployment.Spec.Replicas) {
		deployment.Spec.Replicas = newDeployment.Spec.Replicas
		update = true
	}
	if !reflect.DeepEqual(newDeployment.Spec.Template.Spec.Containers[0].Ports, deployment.Spec.Template.Spec.Containers[0].Ports) {
		deployment.Spec.Template.Spec.Containers[0].Ports = newDeployment.Spec.Template.Spec.Containers[0].Ports
		update = true
	}
	if !reflect.DeepEqual(newDeployment.Spec.Template.Spec.Containers[0].Image, deployment.Spec.Template.Spec.Containers[0].Image) {
		deployment.Spec.Template.Spec.Containers[0].Image = newDeployment.Spec.Template.Spec.Containers[0].Image
		update = true
	}
	if !reflect.DeepEqual(newDeployment.Spec.Template.Spec.Containers[0].Command, deployment.Spec.Template.Spec.Containers[0].Command) {
		deployment.Spec.Template.Spec.Containers[0].Command = newDeployment.Spec.Template.Spec.Containers[0].Command
		update = true
	}
	if !reflect.DeepEqual(newDeployment.Spec.Template.Spec.Containers[0].Args, deployment.Spec.Template.Spec.Containers[0].Args) {
		deployment.Spec.Template.Spec.Containers[0].Args = newDeployment.Spec.Template.Spec.Containers[0].Args
		update = true
	}
	if update {
		log.Info("update deployment")
		return r.Update(ctx, deployment)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dashv1alpha1.DashApplication{}).
		Complete(r)
}
