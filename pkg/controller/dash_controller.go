package controller

import (
	"context"

	"github.com/go-logr/logr"
	dashv1alpha1 "github.com/pluralsh/dash-controller/apis/dash/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.Info("create deployment")
		deployment = genDeployment(dashApp)
		if err := r.Create(ctx, deployment); err != nil {
			return ctrl.Result{}, err
		}
	}

	svc := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, svc); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.Info("create service")
		svc = generateService(dashApp)
		if err := r.Create(ctx, svc); err != nil {
			return ctrl.Result{}, err
		}
	}

	if dashApp.Spec.Ingress != nil {
		ingress := &networkingv1.Ingress{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, ingress); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}

			log.Info("create ingress")
			ingress = genIngress(dashApp)
			if err := r.Create(ctx, ingress); err != nil {
				return ctrl.Result{}, err
			}
		}
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
			Name:      dashApp.Name,
			Namespace: dashApp.Namespace,
			Labels:    baseAppLabels(dashApp.Name, nil),
		},
		Spec: networkingv1.IngressSpec{
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

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dashv1alpha1.DashApplication{}).
		Complete(r)
}
