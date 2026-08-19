from django.http import HttpResponse
from django.shortcuts import render
from django.contrib.auth.decorators import login_required
from .models import Machine
import hashlib
import hmac
import time
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.kdf.hkdf import HKDF

@login_required
def shell(request, machine_name):
    try:
        machine = Machine.objects.get(name=machine_name)
    except Machine.DoesNotExist:
        return HttpResponse("machine not found", status=404)
    
    # get a derived key for signing purposes
    hkdf = HKDF(
        algorithm=hashes.SHA256(),
        length=32,
        salt=None,
        info=info,
    )
    derived_secret_key = hkdf.derive(machine.secret_key)

    # sin a timestamp (get it bc sin -> sine and sine is pronounced the same is sign hahahahahahahahahahaahhahahahahahahaha)
    timestamp = int(time.time())
    derived_key = derive_key(master_secret)
    payload = str(timestamp).encode("utf-8")
    signature = hmac.new(derived_key, payload, hashlib.sha256).hexdigest()

    # [timestamp]-[signature] token, will be split by "-"
    token = f"{str(timestamp)}-{signature}"

    return render(request, 'shell.html', {})