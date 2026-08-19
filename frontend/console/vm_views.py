from django.shortcuts import render, HttpResponse, redirect, reverse
from django.http import StreamingHttpResponse
from .models import Machine, ProxySession
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

vm_proxy_disconnect_host = environ.get("PROXY_DISCONNECT_HOST")
vm_proxy_disconnect_port = environ.get("PROXY_DISCONNECT_PORT")
vm_proxy_disconnect_secret = environ.get("PROXY_DISCONNECT_SECRET")


@login_required
def create_vm(request, machine_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
        machine.info = requests.get(f'http://{machine.hostport}/api/info', headers={'Authorization': f'Bearer {machine.secret_key}'}, timeout=settings.DEFAULT_TIMEOUT).json()
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    except Exception as e:
        return HttpResponse(f"error fetching machine info: {str(e)}", status=500)
    if request.method == 'POST':
        name = request.POST.get('vm_name')
        vcpus = int(request.POST.get('vcpus'))
        memory_mb = float(request.POST.get('memory_mb'))
        boot_file = request.POST.get('boot_file')
        disk_gb = float(request.POST.get('disk_gb'))
        graphics_type = request.POST.get('graphics_type')

        try:
            resp = requests.post(f"http://{machine.hostport}/api/vms/create", headers={'Authorization': f'Bearer {machine.secret_key}'}, json={
                'name': name,
                'vcpus': vcpus,
                'memory_mib': round(memory_mb * 0.9536743),  # convert to MiB
                'boot_file': boot_file,
                'disk_gib': round(disk_gb * 0.9313226), # convert to GiB
                'graphics_type': graphics_type
            })
            if resp.status_code == 200:
                return redirect('vm_detail', machine_name=machine_name, vm_name=name)
            else:
                return HttpResponse(f"Failed to create VM: {resp.text}", status=resp.status_code)
        except Machine.DoesNotExist:
            return HttpResponse("Machine not found", status=404)
    else:
        boot_files = requests.get(f"http://{machine.hostport}/api/vms/boot-files", headers={'Authorization': f'Bearer {machine.secret_key}'}, timeout=settings.DEFAULT_TIMEOUT).json()
        return render(request, 'create_vm.html', {'machine_name': machine_name, 'boot_files': boot_files, 'max_memory_mb': machine.info.get('memory', 0) // 1024 // 1024, 'max_cpus': machine.info.get('cpu_num', 0), 'max_disk_gb': machine.info.get('storage_capacity', 0) // 1024 // 1024 // 1024})

@login_required
def vm_detail(request, machine_name, vm_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        vm = requests.get(f"http://{machine.hostport}/api/vms/get?name={vm_name}", headers={'Authorization': f'Bearer {machine.secret_key}'}, timeout=settings.DEFAULT_TIMEOUT).json()
        if vm.get('error'):
            return HttpResponse(f"error fetching VM details: {vm['error']}", status=503)
    except Exception as e:
        return HttpResponse(f"error fetching VM details: {str(e)}", status=503)
    
    sessions = None
    try:
        sessions = ProxySession.objects.filter(proxy_url=proxy_url_for_vm(machine, vm_name), machine=machine)
    except Exception as e:
        return HttpResponse(f"error fetching sessions: {str(e)}", status=503)

    return render(request, 'vm_detail.html', {'machine': machine, 'vm': vm, 'sessions': sessions, 'error': request.GET.get('error', '')})

@login_required
def vm_start(request, machine_name, vm_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        resp = requests.post(f"http://{machine.hostport}/api/vms/start", headers={'Authorization': f'Bearer {machine.secret_key}'}, json={'name': vm_name})
        if resp.status_code == 200:
            return redirect('vm_detail', machine_name=machine_name, vm_name=vm_name)
        else:
            return HttpResponse(f"Failed to start VM: {resp.text}", status=resp.status_code)
    except Exception as e:
        return HttpResponse(f"error starting VM: {str(e)}", status=503)

@login_required
def vm_stop(request, machine_name, vm_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        force = request.POST.get('force')
        resp = requests.post(f"http://{machine.hostport}/api/vms/stop?force={force}", headers={'Authorization': f'Bearer {machine.secret_key}'}, json={'name': vm_name})
        if resp.status_code == 200:
            return redirect('vm_detail', machine_name=machine_name, vm_name=vm_name)
        else:
            return HttpResponse(f"Failed to stop VM: {resp.text}", status=resp.status_code)
    except Exception as e:
        return HttpResponse(f"error stopping VM: {str(e)}", status=503)

@login_required
def vm_restart(request, machine_name, vm_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        force = request.POST.get('force')
        resp = requests.post(f"http://{machine.hostport}/api/vms/restart?force={force}", headers={'Authorization': f'Bearer {machine.secret_key}'}, json={'name': vm_name})
        if resp.status_code == 200:
            return redirect('vm_detail', machine_name=machine_name, vm_name=vm_name)
        else:
            return HttpResponse(f"Failed to restart VM: {resp.text}", status=resp.status_code)
    except Exception as e:
        return HttpResponse(f"error restarting VM: {str(e)}", status=503)

@login_required
def vm_delete(request, machine_name, vm_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        resp = requests.post(f"http://{machine.hostport}/api/vms/delete", headers={'Authorization': f'Bearer {machine.secret_key}'}, json={'name': vm_name})
        if resp.status_code == 200:
            return redirect('machine_detail', machine_name=machine_name)
        else:
            return HttpResponse(f"Failed to delete VM: {resp.text}", status=resp.status_code)
    except Exception as e:
        return HttpResponse(f"error deleting VM: {str(e)}", status=503)

@login_required
def vm_connect(request, machine_name, vm_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        unclaimedSessions = ProxySession.objects.filter(claimed=False)
        if unclaimedSessions.exists():
            return redirect(reverse('vm_detail', kwargs={'machine_name': machine_name, 'vm_name': vm_name}) + f"?error=there is an unclaimed session for VM {unclaimedSessions.first().vm_name} on machine {unclaimedSessions.first().machine.name}. Please claim or disconnect it first.")
    except Exception as e:
        return HttpResponse(f"error checking unclaimed sessions: {str(e)}", status=503)

    sessionUUID = str(uuid.uuid4())
    session = None
    try:
        ProxySession(proxy_url=proxy_url_for_vm(machine, vm_name), machine=machine, session_id=sessionUUID, initial_req_is_http=False).save()
    except Exception as e:
        return HttpResponse(f"error creating session: {str(e)}", status=503)
    
    return redirect('vm_detail', machine_name=machine_name, vm_name=vm_name)

@login_required
def vm_disconnect(request, machine_name, vm_name, session_id):
    try:
        session = ProxySession.objects.get(session_id=session_id)
        if session.claimed:
            disconnect_session(session_id)
        session.delete()    
    except ProxySession.DoesNotExist:
        return HttpResponse("session not found", status=404)
    return redirect('vm_detail', machine_name=machine_name, vm_name=vm_name)

def disconnect_session(session_id):
    if len(vm_proxy_disconnect_secret) != 128:
        print("error: disconnect secret is not 128 bytes.")
        return
    if len(session_id) != 36:
        print("error: session id is not 36 bytes.")
        return

    try:
        print(f"Connecting to disconnect service at {vm_proxy_disconnect_host}:{vm_proxy_disconnect_port}...")
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.settimeout(5.0) # 5 second timeout should be enough i think
            s.connect((vm_proxy_disconnect_host, int(vm_proxy_disconnect_port)))
            
            s.sendall(vm_proxy_disconnect_secret.encode('utf-8'))
            s.sendall(session_id.encode('utf-8'))
            
            print(f"successfully sent disconnect signal for session: {session_id}")
    except socket.timeout:
        print("timeout: failed to connect to disconnect service")
    except ConnectionRefusedError:
        print("connection refused: disconnect service is not running??")
    except Exception as e:
        print(f"unexpected error occurred: {e}")

def download_iso(request, machine_name):
    machine = None    
    try:
        machine = Machine.objects.get(name=machine_name)
    except Exception as e:
        return HttpResponse(f"error fetching machine: {str(e)}", status=503)
    
    if request.method == 'POST':
        url = request.POST.get('url')
        if not url:
            return HttpResponse("URL is required", status=400)
        
        try:
            resp = requests.post(f"http://{machine.hostport}/api/vms/download-iso", headers={"Content-Type": "application/json", "Authorization": "Bearer " + machine.secret_key}, json={"url": url})
            if resp.status_code != 200:
                return HttpResponse(f"download iso error: {resp.text}", status=504)
        except Exception as e:
            return HttpResponse(f"error downloading ISO: {str(e)}", status=500)

    return render(request, 'download_iso.html', {"machine": machine})

def proxy_url_for_vm(machine, vm_name):
    return f"http://{machine.hostport}/api/vms/{vm_name}/proxy"