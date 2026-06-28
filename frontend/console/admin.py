from django.contrib import admin
from .models import Machine, VMSession
from passkeys.models import UserPasskey
# Register your models here.
admin.site.register(Machine)
admin.site.register(VMSession)
admin.site.register(UserPasskey)