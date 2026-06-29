from django.db import models

# Create your models here.
class Machine(models.Model):
    name = models.CharField(max_length=64, primary_key=True)
    hostport = models.CharField(max_length=512)
    secret_key = models.CharField(max_length=256)

    def __str__(self):
        return self.name

class VMSession(models.Model):
    vm_name = models.CharField(max_length=64)
    machine = models.ForeignKey(Machine, on_delete=models.CASCADE)
    session_id = models.CharField(max_length=36, unique=True)
    claimed = models.BooleanField(default=False)
    claimed_by = models.CharField(max_length=256, null=True, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)

    def __str__(self):
        return f"{self.vm_name} on {self.machine.name}"

# potentially tie a notification to a user
class Notification(models.Model):
    title = models.CharField(max_length=256)
    message = models.TextField()
    read = models.BooleanField(default=False)
    created_at = models.DateTimeField(auto_now_add=True)

    def __str__(self):
        return self.title