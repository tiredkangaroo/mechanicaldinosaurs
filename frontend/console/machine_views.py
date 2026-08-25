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
from . import automation_views

@login_required
def machines(request):
    return redirect('index')

@login_required
def add_machine(request):
    if request.method == 'POST':
        name = request.POST.get('name')
        hostport = request.POST.get('hostport')
        secret_key = request.POST.get('secret_key')

        machine = Machine(name=name, hostport=hostport, secret_key=secret_key)
        machine.save()
        machines = Machine.objects.all()
        return redirect('machine_detail', machine_name=name)
    return render(request, 'add_machine.html')

@login_required
def machine_detail(request, machine_name):
    machine = None
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("Machine not found", status=404)
    try:
        machine.info = requests.get(f'http://{machine.hostport}/api/info', headers={'Authorization': f'Bearer {machine.secret_key}'}, timeout=settings.DEFAULT_TIMEOUT).json()
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
        resp = requests.get(f"http://{machine.hostport}/api/vms/available", headers={'Authorization': f'Bearer {machine.secret_key}'}, timeout=settings.DEFAULT_TIMEOUT)
        if resp.status_code == 503:
            machine.vm_status = "not available ❌"
        else:
            machine.vm_status = "available ✅"
            try:
                machine.vms = requests.get(f"http://{machine.hostport}/api/vms/list", headers={'Authorization': f'Bearer {machine.secret_key}'}, timeout=settings.DEFAULT_TIMEOUT).json()
            except Exception as e:
                machine.vms = {'error': str(e)}
    except Exception as e:
        machine.vm_status = {'error': str(e)}

    # active shell sessions
    try:
        machine.active_shell_sessions = ProxySession.objects.filter(machine=machine, proxy_url=f"http://{machine.hostport}/api/shell", claimed=True)
    except Exception as e:
        machine.active_shell_sessions = {'error': str(e)}
    
    machine.tunnels = machine.cf_tunnels if machine.cf_tunnels else []

    # get the account's tunnel IDs
    account_tunnels = []
    try:
        resp = requests.get(f"https://api.cloudflare.com/client/v4/accounts/{settings.CLOUDFLARE_ACCOUNT_ID}/cfd_tunnel", headers={'Authorization': f'Bearer {settings.CLOUDFLARE_API_TOKEN}'}, timeout=settings.DEFAULT_TIMEOUT)
        if resp.status_code == 200:
            account_tunnels = [{'id': tunnel['id'], 'name': tunnel['name']} for tunnel in resp.json().get('result', [])]
    except Exception as e:
        account_tunnels = {'error': str(e)}

    return render(request, 'machine_detail.html', {'machine': machine, 'account_tunnels': account_tunnels})

@login_required
def machine_delete(request, machine_name):
    try:
        machine = Machine.objects.get(name=machine_name)
        machine.delete()
        machines = Machine.objects.all()
    except Machine.DoesNotExist:
        return HttpResponse("Machine not found", status=404)
    return redirect('index')

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