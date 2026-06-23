from django.shortcuts import render, HttpResponse, redirect
from .models import Machine

# Create your views here.
def index(request):
    return render(request, 'index.html')

def machines(request):
    if request.method == 'POST':
        name = request.POST.get('name')
        hostport = request.POST.get('hostport')
        secret_key = request.POST.get('secret_key')

        machine = Machine(name=name, hostport=hostport, secret_key=secret_key)
        machine.save()
    machines = Machine.objects.all()
    for machine in machines:
        try:
            machine.info = requests.get(f'http://{machine.hostport}/api/info', headers={'Authorization': f'Bearer {machine.secret_key}'}).json()
        except Exception as e:
            machine.info = {'error': str(e)}
    return render(request, 'machines.html', {'machines': machines})


def machine_delete(request, machine_name):
    try:
        machine = Machine.objects.get(name=machine_name)
        machine.delete()
    except Machine.DoesNotExist:
        return HttpResponse("Machine not found", status=404)
    return redirect('machines')