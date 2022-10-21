FROM python:3.7-alpine

USER root

WORKDIR /

COPY app.py app.py
COPY requirements.txt requirements.txt

RUN pip install --trusted-host pypi.python.org -r requirements.txt

EXPOSE 8050

ENV NAME World

CMD ["python", "app.py"]
