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

# note: this file needs to be broken up

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
    try:
        pods_list = kube_core_api.list_pod_for_all_namespaces(watch=False)
        for pod in pods_list.items:
            pods.append({
                "name": pod.metadata.name,
                "namespace": pod.metadata.namespace,
                "status": pod.status.phase,
                "ip": pod.status.pod_ip,
                "created_at": pod.metadata.creation_timestamp,
            })
    except ApiException as e:
        print(f"error fetching pods from k3s cluster: {str(e)}")
    
    # sort pods by status and then by created_at
    pods.sort(key=lambda x: (x['status'], x['created_at'])) # wow thats cool how easy ts was
    
    notifications_unread = Notification.objects.filter(read=False)
    return render(request, 'index.html', {'machines': machines, 'vms': vms, 'num_notifications_unread': len(notifications_unread), 'pods': pods})

def login_view(request):
    if request.method == 'POST':
        username = request.POST.get('username')
        password = request.POST.get('password')
        user = authenticate(request, username=username, password=password)
        if user is not None and user.is_active:
            login(request, user)
            next_url = request.POST.get('next')
            print(f"user authenticated successfully, redirect? {next_url}")
            if next_url:
                return redirect(next_url)
            else:
                return redirect('index')
        else:
            return render(request, 'login.html', {'error': 'invalid username/password or deactivated account'})
    return render(request, 'login.html')

def logout_view(request):
    logout(request)
    return redirect('login')

@login_required
def passkeys_mgmt(request):
    keys = UserPasskey.objects.filter(user=request.user)
    return render(request, 'passkeys_mgmt.html', {'keys': keys})

@login_required
def machines(request):
    if request.method == 'POST':
        name = request.POST.get('name')
        hostport = request.POST.get('hostport')
        secret_key = request.POST.get('secret_key')

        machine = Machine(name=name, hostport=hostport, secret_key=secret_key)
        machine.save()
        return redirect('machine_detail', machine_name=name)
    return redirect('index')

@login_required
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

    return render(request, 'machine_detail.html', {'machine': machine})

@login_required
def machine_delete(request, machine_name):
    try:
        machine = Machine.objects.get(name=machine_name)
        machine.delete()
    except Machine.DoesNotExist:
        return HttpResponse("Machine not found", status=404)
    return redirect('index')

@login_required
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

@login_required
def vm_detail(request, machine_name, vm_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        vm = requests.get(f"http://{machine.hostport}/api/vms/get?name={vm_name}", headers={'Authorization': f'Bearer {machine.secret_key}'}).json()
        if vm.get('error'):
            return HttpResponse(f"error fetching VM details: {vm['error']}", status=503)
    except Exception as e:
        return HttpResponse(f"error fetching VM details: {str(e)}", status=503)
    
    sessions = None
    try:
        sessions = VMSession.objects.filter(vm_name=vm_name, machine=machine)
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

@login_required
def vm_disconnect(request, machine_name, vm_name, session_id):
    try:
        session = VMSession.objects.get(session_id=session_id)
        if session.claimed:
            disconnect_session(session_id)
        session.delete()    
    except VMSession.DoesNotExist:
        return HttpResponse("session not found", status=404)
    return redirect('vm_detail', machine_name=machine_name, vm_name=vm_name)

@login_required
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

@login_required
def container_detail(request, machine_name, container_id):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    container = None
    try:
        container = requests.get(f"http://{machine.hostport}/api/containers/get?id={container_id}", headers={'Authorization': f'Bearer {machine.secret_key}'}).json()
    except Exception as e:
        return HttpResponse(f"error fetching container details: {str(e)}", status=503)
    
    # 2026-03-20T19:48:39.511080907Z -> 
    container['Container']['created_human'] = parser.parse(container['Container']['Created']).strftime("%B %d, %Y, %H:%M:%S")

    container['Container']['compose_svc'] = container['Container']['Config']['Labels'].get('com.docker.compose.service', None)
    return render(request, 'container_detail.html', {'machine': machine, 'container': container['Container']})

@login_required
def container_start(request, machine_name, container_id):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        requests.post(f"http://{machine.hostport}/api/containers/start", headers={'Authorization': f'Bearer {machine.secret_key}'}, json={'id': container_id})
    except Exception as e:
        return HttpResponse(f"error starting container: {str(e)}", status=503)
    return redirect('container_detail', machine_name=machine_name, container_id=container_id)

@login_required
def container_stop(request, machine_name, container_id):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        requests.post(f"http://{machine.hostport}/api/containers/stop", headers={'Authorization': f'Bearer {machine.secret_key}'}, json={'id': container_id})
    except Exception as e:
        return HttpResponse(f"error stopping container: {str(e)}", status=503)
    return redirect('container_detail', machine_name=machine_name, container_id=container_id)

@login_required 
def container_remove(request, machine_name, container_id):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    try:
        requests.post(f"http://{machine.hostport}/api/containers/remove", headers={'Authorization': f'Bearer {machine.secret_key}'}, json={'id': container_id})
    except Exception as e:
        return HttpResponse(f"error removing container: {str(e)}", status=503)
    return redirect('machine_detail', machine_name=machine_name)

@login_required
def stream_container_logs(request, machine_name, container_id):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    api_url = f"http://{machine.hostport}/api/containers/{container_id}/logs"

    def log_generator():
        try:
            with requests.get(api_url, headers={'Authorization': f'Bearer {machine.secret_key}'}, stream=True, timeout=None) as response:
                if response.status_code != 200:
                    yield f"error from server: {response.text}".encode('utf-8')
                    return

                # keep sending over chunks
                for chunk in response.iter_content(chunk_size=1024):
                    if chunk:
                        yield chunk
        except requests.exceptions.RequestException as e:
            yield f"error: {str(e)}".encode('utf-8')

    return StreamingHttpResponse(log_generator(), content_type="text/plain")

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

# this function was mostly generated by ai
@login_required
def pod_detail(request, namespace, pod_name):
    try:
        pod = kube_core_api.read_namespaced_pod(name=pod_name, namespace=namespace)
    except ApiException as e:
        return HttpResponse(f"error fetching pod details: {str(e)}", status=503)

    # index container statuses by name for lookup
    container_statuses = {}
    for cs in (pod.status.container_statuses or []):
        state = cs.state
        if state.running:
            cs_state = {"status": "running", "started_at": state.running.started_at}
        elif state.waiting:
            cs_state = {"status": "waiting", "reason": state.waiting.reason}
        elif state.terminated:
            cs_state = {"status": "terminated", "exit_code": state.terminated.exit_code, "reason": state.terminated.reason}
        else:
            cs_state = {"status": "unknown"}
        container_statuses[cs.name] = {
            "ready": cs.ready,
            "restart_count": cs.restart_count,
            "image_id": cs.image_id,
            "state": cs_state,
        }

    pod_info = {
        "name": pod.metadata.name,
        "namespace": pod.metadata.namespace,
        "status": pod.status.phase,
        "ip": pod.status.pod_ip,
        "host_ip": pod.status.host_ip,
        "node": pod.spec.node_name,
        "created_at": pod.metadata.creation_timestamp,
        "start_time": pod.status.start_time,
        "labels": pod.metadata.labels or {},
        "service_account": pod.spec.service_account_name,
        "restart_policy": pod.spec.restart_policy,
        "conditions": [
            {"type": c.type, "status": c.status, "last_transition": c.last_transition_time}
            for c in (pod.status.conditions or [])
        ],
        "containers": [],
    }

    for container in pod.spec.containers:
        def parse_env(env_var):
            if env_var.value is not None:
                return env_var.value
            if env_var.value_from:
                vf = env_var.value_from
                if vf.field_ref:
                    return f"<fieldRef: {vf.field_ref.field_path}>"
                if vf.secret_key_ref:
                    return f"<secretKeyRef: {vf.secret_key_ref.name}/{vf.secret_key_ref.key}>"
                if vf.config_map_key_ref:
                    return f"<configMapKeyRef: {vf.config_map_key_ref.name}/{vf.config_map_key_ref.key}>"
                if vf.resource_field_ref:
                    return f"<resourceFieldRef: {vf.resource_field_ref.resource}>"
            return "<unknown>"

        resources = container.resources or {}
        cs = container_statuses.get(container.name, {})

        container_info = {
            "name": container.name,
            "image": container.image,
            "command": container.command or [],
            "ports": [port.container_port for port in (container.ports or [])],
            "env": {env.name: parse_env(env) for env in (container.env or [])},
            "volume_mounts": [
                {"path": vm.mount_path, "name": vm.name, "read_only": vm.read_only}
                for vm in (container.volume_mounts or [])
            ],
            "resources": {
                "requests": getattr(resources, "requests", None) or {},
                "limits": getattr(resources, "limits", None) or {},
            },
            "status": cs,
        }
        pod_info["containers"].append(container_info)

    return render(request, 'pod_detail.html', {'pod': pod_info})

def pod_deploy(request):
    if request.method == 'POST':
        name = request.POST.get('name').lower().strip()
        image = request.POST.get('image')
        
        # resource stuff
        cpu_resource_request = request.POST.get('cpuResourceRequest')
        memory_resource_request = request.POST.get('memoryResourceRequest')
        cpu_resource_limit = request.POST.get('cpuResourceLimit')
        memory_resource_limit = request.POST.get('memoryResourceLimit')

        replicas = int(request.POST.get('replicas', 1))
        namespace = request.POST.get('namespace', 'default')
        env_vars = request.POST.get('env_vars') # format: k1=v1,k2=v2

        # we have to create a service alongside the deployment if there's any ports to expose
        expose_externally = request.POST.get('expose_externally') == 'true'
        ports = request.POST.get('ports') # map of containerPort:hostPort or containerPort:0 if not to be exposed but should still be marked to kube

        env = []
        if env_vars_raw: # i probably don't need to check this but just in case
            for item in env_vars_raw.split(','):
                if '=' in item:
                    k, v = item.split('=', 1)
                    env.append(client.V1EnvVar(name=k.strip(), value=v.strip()))
                else:
                    return HttpResponse(f"invalid env var format: {item}", status=400) # shouldn't happen if frontend works correctly
        
        container_ports = []
        service_ports = []

        if ports_raw: # again probably don't need to check this lol
            for mapping in ports_raw.split(','):
                if ':' in mapping:
                    c_port, s_port = mapping.split(':')
                    c_port, s_port = int(c_port.strip()), int(s_port.strip())

                    # mark the container port on the container spec 
                    container_ports.append(client.V1ContainerPort(container_port=c_port))

                    # if a service port (for exposing) has been specified, add it                    
                    if s_port > 0:
                        service_ports.append(client.V1ServicePort(
                            name=f"port-{c_port}", # name for the port will be the container port it is mapped to
                            port=s_port,         # Port exposed on the Service network
                            target_port=c_port   # Target port inside the container
                        ))

        # resource requests and limits
        resources = client.V1ResourceRequirements(
            requests={"cpu": cpu_resource_request, "memory": memory_resource_request},
            limits={"cpu": cpu_resource_limit, "memory": memory_resource_limit}
        )

        # container spec -> pod templ spec -> deployment spec -> deployment

        container = client.V1Container(
            name=f"{name}-container",
            image=image,
            ports=container_ports if container_ports else None,
            env=env if env else None,
            resources=resources
        )

        template = client.V1PodTemplateSpec(
            metadata=client.V1ObjectMeta(labels={"app": name}),
            spec=client.V1PodSpec(containers=[container])
        )
        
        deployment_spec = client.V1DeploymentSpec(
            replicas=replicas,
            selector=client.V1LabelSelector(match_labels={"app": name}),
            template=template
        )
        
        deployment = client.V1Deployment(
            api_version="apps/v1",
            kind="Deployment",
            metadata=client.V1ObjectMeta(name=f"{name}-deployment"),
            spec=deployment_spec
        )

        # boom wow a deployment
        try:
            apps_v1.create_namespaced_deployment(namespace=namespace, body=deployment)
        except ApiException as e:
            return HttpResponse(f"error creating deployment: {str(e)}", status=503)

        # if we have service ports, create a service
        if service_ports:
            service_spec = client.V1ServiceSpec(
                selector={"app": name},
                ports=service_ports,
                type="LoadBalancer" if expose_externally else "ClusterIP"
            )

            service = client.V1Service(
                api_version="v1",
                kind="Service",
                metadata=client.V1ObjectMeta(name=f"{name}-service"),
                spec=service_spec
            )

            try:
                core_v1.create_namespaced_service(namespace=namespace, body=service)
            except ApiException as e:
                return HttpResponse(f"error creating service: {str(e)}", status=503) # imagine getting this far just to have the service creation fail

    return render(request, 'pod_deploy.html')

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
    