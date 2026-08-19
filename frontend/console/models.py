from django.db import models
from django.contrib.auth.models import User

# Create your models here.
class Machine(models.Model):
    name = models.CharField(max_length=64, primary_key=True)
    hostport = models.CharField(max_length=512)
    secret_key = models.CharField(max_length=256)

    def __str__(self):
        return self.name

class ProxySession(models.Model):
    session_id = models.CharField(max_length=36, unique=True)
    machine = models.ForeignKey(Machine, on_delete=models.CASCADE)
    proxy_url = models.CharField(max_length=512)
    claimed = models.BooleanField(default=False)
    claimed_by = models.CharField(max_length=256, null=True, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)

    def __str__(self):
        return f"{self.session_id} on {self.machine.name}"

# this is NOT the source of truth for deployments; this just holds info that would not persist bc they're modified underlying (the replicas desired, when you stop it) and the service name (which needs a concrete way of referencing it)
class Deployment(models.Model):
    name = models.CharField(max_length=256)
    replicas_desired = models.IntegerField()
    service_name = models.CharField(max_length=256, null=True)

    def __str__(self):
        return f"{self.name} in {self.namespace}"

# going with json bc easier
class Automation(models.Model):
    automation_id = models.CharField(max_length=36, primary_key=True)
    json_data = models.JSONField() # automationcommunicable (automation_engine/automation.go:130, symbol: AutomationCommunicable)

class AuditLog(models.Model):
    action = models.CharField(max_length=256)
    detail = models.TextField()
    ipaddr = models.CharField(max_length=1024)
    timestamp = models.DateTimeField(auto_now_add=True)
    user = models.ForeignKey(User, on_delete=models.PROTECT, null=True)

    def __str__(self):
        return f"{self.action} from {self.ipaddr} at {self.timestamp} by {self.user}"

# class Automation(models.Model):
#     automation_id = models.CharField(max_length=36, primary_key=True)
#     enabled = models.BooleanField(default=False)
    
#     trigger_type = models.CharField(max_length=64) # time, interval, machines info data refresh
#     trigger_time = models.DateTimeField(null=True) # only used if trigger_type is "time"
#     trigger_every = models.DurationField(null=True) # only used if trigger_type is "interval"

#     condition_exists = models.BooleanField(default=False) # condition is optional
#     condition_variable = models.CharField(max_length=64, null=True)
#     condition_operator = models.CharField(max_length=64, null=True)
#     condition_value = models.CharField(max_length=256, null=True)
#     condition_negate = models.BooleanField(default=False, null=True)

#     action_type = models.CharField(max_length=64) # email

#     action_email_to = models.CharField(max_length=256, null=True) # only used if action_type is "email"
#     action_email_subject = models.CharField(max_length=256, null=True) # only used if action_type is "email"
#     action_email_body = models.TextField(null=True) # only used if action_type is "email"