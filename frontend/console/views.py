from django.shortcuts import render
from .models import Machine

# Create your views here.
def index(request):
    return render(request, 'console/index.html')

def list_machines(request):
    machines = Machine.objects.all()
    return render(request, 'console/machines.html', {'machines': machines})