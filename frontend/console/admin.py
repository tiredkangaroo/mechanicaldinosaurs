from django.contrib import admin
from .models import Machine, ProxySession, AuditLog
from passkeys.models import UserPasskey
from django.contrib.sessions.models import Session

# Register your models here.
admin.site.register(Machine)
admin.site.register(ProxySession)
admin.site.register(UserPasskey)
admin.site.register(Session)
admin.site.register(AuditLog)