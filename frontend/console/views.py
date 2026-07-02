from django.shortcuts import render, HttpResponse, redirect, reverse
from django.http import StreamingHttpResponse
from .models import Machine, VMSession, Notification
import requests, math, uuid, socket
from dateutil import parser 
from django.contrib.auth.decorators import login_required
from django.contrib.auth import authenticate, login, logout
from passkeys.models import UserPasskey
from os import environ, path
from kubernetes import client, config
from kubernetes.client.rest import ApiException
from django.conf import settings
import resend
from django.views.decorators.clickjacking import xframe_options_exempt

# note: organize imports

# note: make these configurable
vm_proxy_disconnect_host = "localhost"
vm_proxy_disconnect_port = 3832  
vm_proxy_disconnect_secret = "08ba70bc9cb9a486ed0cdc7798e9fb571f75f9e6888d40f07f076679f0781a4ebda8df3a2c6cb2d4bae434cbcbfc3300020214d79c97645daabef348b1bb4c8f"

resend.api_key = environ.get("RESEND_API_KEY")

k3_config_path = path.join(settings.BASE_DIR, 'k3s-config.yaml')
config.load_kube_config(config_file=k3_config_path)
kube_core_api = client.CoreV1Api()
kube_app_api = client.AppsV1Api()

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
            except Exception as e:
                print(f"error fetching vms for machine {machine.name}: {str(e)}")
    
    deployments = []
    try:
        deployments_list = kube_app_api.list_deployment_for_all_namespaces(watch=False)
        
        for deploy in deployments_list.items:
            # get desired and ready replicas (wow kaboom kablow)
            desired_replicas = deploy.spec.replicas or 0
            ready_replicas = deploy.status.ready_replicas or 0
            
            # friendly status string based on replica matching
            # wow can i be friends with you, mr. friendly status string?
            # i want new friends but they don't want me (the strokes)
            if ready_replicas == desired_replicas:
                if ready_replicas == 0:
                    status = "Done"
                else:   
                    status = "Healthy"
            elif ready_replicas == 0: # oop
                status = "Critical"
            else:
                status = "Scaling"

            deployments.append({
                "name": deploy.metadata.name,
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
    
    notifications_unread = Notification.objects.filter(read=False)
    return render(request, 'index.html', {'machines': machines, 'vms': vms, 'num_notifications_unread': len(notifications_unread), 'deployments': deployments})

# test view; not to be used in prod
@login_required
def send_email(request):
    if request.method == 'POST':
        subject = request.POST.get('subject')
        message = request.POST.get('message')
        recipient = request.POST.get('recipient')
        try:
            resend.Emails.send(resend.Emails.SendParams({
                "from": "Infrastructure <infra@mechanicaldinosaurs.net>",
                "to": [recipient],
                "subject": subject,
                "text": message,
            }))
        except Exception as e:
            print(f"error sending email: {str(e)}")
            return HttpResponse("error sending email: " + str(e), status=500)
    return render(request, 'send_email.html')

@login_required
def notifications(request):
    notifications = Notification.objects.all().order_by('-created_at')
    return render(request, 'notifications.html', {'notifications': notifications})
