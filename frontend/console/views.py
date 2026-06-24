from django.shortcuts import render, HttpResponse, redirect, reverse
from .models import Machine, VMSession
import requests, math, uuid, socket

# note: secure this whole thing with auth

# note: make these configurable
vm_proxy_disconnect_host = "localhost"
vm_proxy_disconnect_port = 3832  
vm_proxy_disconnect_secret = "08ba70bc9cb9a486ed0cdc7798e9fb571f75f9e6888d40f07f076679f0781a4ebda8df3a2c6cb2d4bae434cbcbfc3300020214d79c97645daabef348b1bb4c8f"

# Create your views here.
def index(request):
    return render(request, 'index.html')


def machines(request):
    if request.method == 'POST':
        name = request.POST.get('name')
        hostport = request.POST.get('hostport')
        secret_key = request.POST.get('secret_key')

        machine = Machine(name=name, hostport=hostport, secret_key=secret_key)
        machine.save()
    machines = Machine.objects.all()
    for machine in machines:
        try:
            machine.info = requests.get(f'http://{machine.hostport}/api/info', headers={'Authorization': f'Bearer {machine.secret_key}'}).json()
        except Exception as e:
            machine.info = {'error': str(e)}
    return render(request, 'machines.html', {'machines': machines})

def machine_detail(request, machine_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("Machine not found", status=404)
    try:
        machine.info = requests.get(f'http://{machine.hostport}/api/info', headers={'Authorization': f'Bearer {machine.secret_key}'}).json()
        machine.info['uptime'] = format_seconds(machine.info.get('uptime', 0))
        machine.info['memory_util'] = f"{( float(machine.info.get('memory_used', 0)) / float(machine.info.get('memory', 1))) * 100:.2f}%"
        machine.info['storage_util'] = f"{(float(machine.info.get('storage_used', 0)) / float(machine.info.get('storage_capacity', 1))) * 100:.2f}%"
        machine.info['memory_used'] = format_bytes(machine.info.get('memory_used', 0))
        machine.info['memory'] = format_bytes(machine.info.get('memory', 0))
        machine.info['storage_used'] = format_bytes(machine.info.get('storage_used', 0))
        machine.info['storage_capacity'] = format_bytes(machine.info.get('storage_capacity', 0))
    except Exception as e:
        machine.info = {'error': str(e)}
    
    try:
        resp = requests.get(f"http://{machine.hostport}/api/vms/available", headers={'Authorization': f'Bearer {machine.secret_key}'})
        if resp.status_code == 503:
            machine.vm_status = "not available ❌"
        else:
            machine.vm_status = "available ✅"
            try:
                machine.vms = requests.get(f"http://{machine.hostport}/api/vms/list", headers={'Authorization': f'Bearer {machine.secret_key}'}).json()
            except Exception as e:
                machine.vms = {'error': str(e)}
    except Exception as e:
        machine.vm_status = {'error': str(e)}

    try:
        resp = requests.get(f"http://{machine.hostport}/api/docker/available", headers={'Authorization': f'Bearer {machine.secret_key}'})
        if resp.status_code == 503:
            machine.docker_status = "not available ❌"
        else:
            machine.docker_status = "available ✅"
            try:
                machine.containers = requests.get(f"http://{machine.hostport}/api/containers/list", headers={'Authorization': f'Bearer {machine.secret_key}'}).json()
            except Exception as e:
                machine.containers = {'error': str(e)}
    except Exception as e:
        machine.docker_status = {'error': str(e)}
    

    return render(request, 'machine_detail.html', {'machine': machine})

def machine_delete(request, machine_name):
    try:
        machine = Machine.objects.get(name=machine_name)
        machine.delete()
    except Machine.DoesNotExist:
        return HttpResponse("Machine not found", status=404)
    return redirect('machines')

def create_vm(request, machine_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
        machine.info = requests.get(f'http://{machine.hostport}/api/info', headers={'Authorization': f'Bearer {machine.secret_key}'}).json()
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
        boot_files = requests.get(f"http://{machine.hostport}/api/vms/boot-files", headers={'Authorization': f'Bearer {machine.secret_key}'}).json()
        return render(request, 'create_vm.html', {'machine_name': machine_name, 'boot_files': boot_files, 'max_memory_mb': machine.info.get('memory', 0) // 1024 // 1024, 'max_cpus': machine.info.get('cpu_num', 0), 'max_disk_gb': machine.info.get('storage_capacity', 0) // 1024 // 1024 // 1024})

def vm_detail(request, machine_name, vm_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        vm = requests.get(f"http://{machine.hostport}/api/vms/get?name={vm_name}", headers={'Authorization': f'Bearer {machine.secret_key}'}).json()
    except Exception as e:
        return HttpResponse(f"error fetching VM details: {str(e)}", status=503)
    
    sessions = None
    try:
        sessions = VMSession.objects.filter(vm_name=vm_name, machine=machine)
    except Exception as e:
        return HttpResponse(f"error fetching sessions: {str(e)}", status=503)

    return render(request, 'vm_detail.html', {'machine': machine, 'vm': vm, 'sessions': sessions, 'error': request.GET.get('error', '')})

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

def vm_connect(request, machine_name, vm_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        unclaimedSessions = VMSession.objects.filter(claimed=False)
        if unclaimedSessions.exists():
            return redirect(reverse('vm_detail', kwargs={'machine_name': machine_name, 'vm_name': vm_name}) + f"?error=there is an unclaimed session for VM {unclaimedSessions.first().vm_name} on machine {unclaimedSessions.first().machine.name}. Please claim or disconnect it first.")
    except Exception as e:
        return HttpResponse(f"error checking unclaimed sessions: {str(e)}", status=503)

    sessionUUID = str(uuid.uuid4())
    session = None
    try:
        VMSession(vm_name=vm_name, machine=machine, session_id=sessionUUID).save()
    except Exception as e:
        return HttpResponse(f"error creating session: {str(e)}", status=503)
    
    return redirect('vm_detail', machine_name=machine_name, vm_name=vm_name)

def vm_disconnect(request, machine_name, vm_name, session_id):
    try:
        session = VMSession.objects.get(session_id=session_id)
        if session.claimed:
            disconnect_session(session_id)
        session.delete()    
    except VMSession.DoesNotExist:
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
            s.connect((vm_proxy_disconnect_host, vm_proxy_disconnect_port))
            
            s.sendall(vm_proxy_disconnect_secret.encode('utf-8'))
            s.sendall(session_id.encode('utf-8'))
            
            print(f"successfully sent disconnect signal for session: {session_id}")
    except socket.timeout:
        print("timeout: failed to connect to disconnect service")
    except ConnectionRefusedError:
        print("connection refused: disconnect service is not running??")
    except Exception as e:
        print(f"unexpected error occurred: {e}")

# util funcs

def format_bytes(bytes):
    if bytes == 0:
        return "0 Bytes"
    size_name = ("Bytes", "KB", "MB", "GB", "TB")
    i = int(math.floor(math.log(bytes, 1024)))
    p = math.pow(1024, i)
    s = round(bytes / p, 2)
    return f"{s} {size_name[i]}"

def format_seconds(seconds):
    # x days, y hours, z minutes, w seconds
    # x hours, y minutes, z seconds
    # x minutes, y seconds
    # x seconds
    days, seconds = divmod(seconds, 86400)
    hours, seconds = divmod(seconds, 3600)
    minutes, seconds = divmod(seconds, 60)
    if days > 0:
        return f"{days} days, {hours} hours, {minutes} minutes, {seconds} seconds"
    elif hours > 0:
        return f"{hours} hours, {minutes} minutes, {seconds} seconds"
    elif minutes > 0:
        return f"{minutes} minutes, {seconds} seconds"
    else:
        return f"{seconds} seconds"
    