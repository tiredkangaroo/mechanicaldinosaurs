from django.shortcuts import render
from .models import Machine

# Create your views here.
def index(request):
    return render(request, 'index.html')

def list_machines(request):
    machines = Machine.objects.all()
    return render(request, 'machines.html', {'machines': machines})