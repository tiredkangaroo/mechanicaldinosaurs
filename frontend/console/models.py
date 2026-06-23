from django.db import models

# Create your models here.
class Machine(models.Model):
    name = models.CharField(max_length=64)
    hostport = models.CharField(max_length=512)
    secret_key = models.CharField(max_length=256)

    def __str__(self):
        return self.name