from django.shortcuts import render, HttpResponse, redirect, reverse
from django.http import StreamingHttpResponse
from .models import Machine, VMSession, Automation
import requests, math, uuid, socket, urllib3
from dateutil import parser 
from django.contrib.auth.decorators import login_required
from django.contrib.auth import authenticate, login, logout
from passkeys.models import UserPasskey
from os import environ, path
from kubernetes import client, config
from kubernetes.client.rest import ApiException
from django.conf import settings
from django.views.decorators.clickjacking import xframe_options_exempt
from django.utils.safestring import mark_safe

from . import k3
from . import automation_views
# note: organize imports

# Create your views here.

# note: fix this
automation_engine_url = environ.get("AUTOMATION_ENGINE_URL")
automation_engine_secret = environ.get("AUTOMATION_ENGINE_SECRET")


@login_required
def index(request):
    machines = Machine.objects.all()
    vms = []
    containers = []
    pods = []
    print("getting vms from all machines")
    for machine in machines:
            try:
                machine.vms = requests.get(f"http://{machine.hostport}/api/vms/list", headers={'Authorization': f'Bearer {machine.secret_key}'}, timeout=settings.DEFAULT_TIMEOUT).json()
                if not getattr(machine.vms, 'error', None): # not err is present
                    if machine.vms:
                        for vm in machine.vms:
                            vm['machine'] = machine
                            vms.append(vm)
                machine.status = "reachable"
            except Exception as e:
                print(f"error fetching vms for machine {machine.name}: {str(e)}")
                machine.status = "unreachable"

    print("getting deployments for all namespaces") 
    deployments_by_namespace = {}
    try:
        deployments_list = k3.kube_app_api.list_deployment_for_all_namespaces(watch=False, _request_timeout=1)
        
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


            if deploy.metadata.namespace not in deployments_by_namespace:
                deployments_by_namespace[deploy.metadata.namespace] = []
            deployments_by_namespace[deploy.metadata.namespace].append({
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
    except urllib3.exceptions.MaxRetryError as e:
        print(f"max retry error: {str(e)}")

    # sort by status (crititcal > scaling > healthy) and then by created at for each namespace
    for namespace, deploys in deployments_by_namespace.items():
        deploys.sort(key=lambda x: (x['status'], x['created_at']))

    print("pulling automations from db")
    automations = []
    try:
        automations = [a.json_data for a in Automation.objects.all()]
    except Exception as e:
        print(f"error fetching automations: {str(e)}")

    operation_to_symbol = {
        "greater than": ">",
        "less than": "<",
        "equals": "==",
        "greater than or equal": ">=",
        "less than or equal": "<=",
    }
    for automation in automations:
        if automation["trigger"]["type"] == "time":
            human_readable_time = parser.parse(automation["trigger"]["time"]).strftime("%Y-%m-%d %H:%M:%S")
            automation["trigger"]["name"] = mark_safe(f"<b>at</b> {automation['trigger']['time']}")
        elif automation["trigger"]["type"] == "interval":
            automation["trigger"]["name"] = mark_safe(f"<b>every</b> {automation['trigger']['every']} <b>seconds</b>")
        elif automation["trigger"]["type"] == "machines info refresh":
            automation["trigger"]["name"] = mark_safe(f"<b>on</b> machine info refresh")
        
        automation["condition"]["name"] = mark_safe(f"<div style='display: flex; flex-direction: row; gap: 2px; width: 100%;'>{'<b>NOT</b> ' if automation['condition']['not'] else ''}<pre>{automation['condition']['variable']}</pre> <p>{operation_to_symbol.get(automation['condition']['op'], automation['condition']['op'])} {automation['condition']['value']}</p></div>")

        if automation["action"]["type"] == "email":
            automation["action"]["name"] = mark_safe(f"<b>send email to</b> {automation['action']['email']['to']}")
        if automation["action"]["type"] == "slack":
            automation["action"]["name"] = mark_safe(f"<b>send slack message to </b><code>{automation['action']['slack']['conversation_id']}</code>")

    return render(request, 'index.html', {'machines': machines, 'vms': vms, 'deployments_by_ns': deployments_by_namespace, 'automations': automations})