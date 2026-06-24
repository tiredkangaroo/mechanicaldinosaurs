from django.contrib import admin
from .models import Machine, VMSession

# Register your models here.
admin.site.register(Machine)
admin.site.register(VMSession)