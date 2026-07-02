from django.shortcuts import render, HttpResponse, redirect, reverse
from django.http import StreamingHttpResponse
from .models import Machine, VMSession
import requests, math, uuid, socket
from dateutil import parser 
from django.contrib.auth.decorators import login_required
from django.contrib.auth import authenticate, login, logout
from passkeys.models import UserPasskey
from os import environ, path
from kubernetes import client, config
from kubernetes.client.rest import ApiException
from django.conf import settings
from django.views.decorators.clickjacking import xframe_options_exempt
from . import k3
# note: organize imports

# Create your views here.

@login_required
def index(request):
    machines = Machine.objects.all()
    vms = []
    containers = []
    pods = []
    for machine in machines:
            try:
                machine.vms = requests.get(f"http://{machine.hostport}/api/vms/list", headers={'Authorization': f'Bearer {machine.secret_key}'}).json()
                if not getattr(machine.vms, 'error', None): # not err is present
                    for vm in machine.vms:
                        vm['machine'] = machine
                        vms.append(vm)
                machine.status = "reachable"
            except Exception as e:
                print(f"error fetching vms for machine {machine.name}: {str(e)}")
                machine.status = "unreachable"
    
    deployments = []
    try:
        deployments_list = k3.kube_app_api.list_deployment_for_all_namespaces(watch=False)
        
        for deploy in deployments_list.items:
            # get desired and ready replicas (wow kaboom kablow)
            desired_replicas = deploy.spec.replicas or 0
            ready_replicas = deploy.status.ready_replicas or 0
            
            # friendly status string based on replica matching
            # wow can i be friends with you, mr. friendly status string?
            # i want new friends but they don't want me (the strokes)
            if ready_replicas == desired_replicas:
                if ready_replicas == 0:
                    status = "Stopped"
                else:   
                    status = "Healthy"
            elif ready_replicas == 0: # oop
                status = "Critical"
            elif ready_replicas < desired_replicas:
                status = "Scaling Up"
            elif ready_replicas > desired_replicas and desired_replicas > 0:
                status = "Scaling Down"
            elif desired_replicas == 0 and ready_replicas > 0:
                status = "Stopping"
            else:
                status = "Unknown"

            deployments.append({
                "name": deploy.metadata.name,
                "display_name": deploy.metadata.name.replace("-deployment", ""),
                "namespace": deploy.metadata.namespace,
                "replicas_desired": desired_replicas,
                "replicas_ready": ready_replicas,
                "status": status,
                "image": deploy.spec.template.spec.containers[0].image if deploy.spec.template.spec.containers else "Unknown",
                "created_at": deploy.metadata.creation_timestamp,
            })
    except ApiException as e:
        print(f"error fetching deployments from cluster: {str(e)}")

    # sort by namespace and then by status (crititcal > scaling > healthy) and then by created at
    deployments.sort(key=lambda x: (x['namespace'], x['status'], x['created_at']))
    
    return render(request, 'index.html', {'machines': machines, 'vms': vms, 'deployments': deployments})