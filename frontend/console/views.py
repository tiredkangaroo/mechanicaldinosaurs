from django.shortcuts import render, HttpResponse, redirect
from .models import Machine
import requests, math

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
    except Exception as e:
        machine.vm_status = {'error': str(e)}

    try:
        resp = requests.get(f"http://{machine.hostport}/api/docker/available", headers={'Authorization': f'Bearer {machine.secret_key}'})
        if resp.status_code == 503:
            machine.docker_status = "not available ❌"
        else:
            machine.docker_status = "available ✅"
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
    