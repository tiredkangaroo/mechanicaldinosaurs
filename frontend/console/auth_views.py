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
import resend
from django.views.decorators.clickjacking import xframe_options_exempt

def login_view(request):
    if request.user.is_authenticated:
        return redirect('index')
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
