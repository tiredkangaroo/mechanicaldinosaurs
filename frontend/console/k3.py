from os import path
from django.conf import settings
from kubernetes import client, config

k3_config_path = path.join(settings.BASE_DIR, 'k3s-config.yaml')
config.load_kube_config(config_file=k3_config_path)
kube_core_api = client.CoreV1Api()
kube_app_api = client.AppsV1Api()