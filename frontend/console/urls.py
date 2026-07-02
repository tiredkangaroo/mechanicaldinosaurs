from django.urls import path
from . import views
from . import auth_views
from . import machine_views
from . import vm_views
from . import k3_views

urlpatterns = [
    path('', views.index, name='index'),
    path('login/', auth_views.login_view, name='login'),
    path('passkeys-mgmt', auth_views.passkeys_mgmt, name='passkeys_mgmt'),
    path('logout/', auth_views.logout_view, name='logout'),
    path('machines/', machine_views.machines, name='machines'),
    path('machine/add/', machine_views.add_machine, name='add_machine'),
    # perhaps find some way to invalidate "add" being the name of a machine lol
    path('machine/<str:machine_name>/', machine_views.machine_detail, name='machine_detail'),
    path('machine/<str:machine_name>/delete/', machine_views.machine_delete, name='machine_delete'),
    path("machine/<str:machine_name>/vms/create", vm_views.create_vm, name="create_vm"),
    path("machine/<str:machine_name>/vms/<str:vm_name>", vm_views.vm_detail, name="vm_detail"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/start", vm_views.vm_start, name="vm_start"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/stop", vm_views.vm_stop, name="vm_stop"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/restart", vm_views.vm_restart, name="vm_restart"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/delete", vm_views.vm_delete, name="vm_delete"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/connect", vm_views.vm_connect, name="vm_connect"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/<str:session_id>/disconnect", vm_views.vm_disconnect, name="vm_disconnect"), 
    path('k3/deploy/', k3_views.deploy, name='deploy'),
    path('k3/deployments/<str:namespace>/<str:deployment_name>/', k3_views.deployment_detail, name='deployment_detail'),
    path('k3/deployments/<str:namespace>/<str:deployment_name>/start', k3_views.start_deployment, name='start_deployment'),
    path('k3/deployments/<str:namespace>/<str:deployment_name>/stop', k3_views.stop_deployment, name='stop_deployment'),
    path('k3/deployments/<str:namespace>/<str:deployment_name>/delete', k3_views.delete_deployment, name='delete_deployment'),
    
    path('k3/pod/<str:namespace>/<str:pod_name>/logs', k3_views.pod_logs, name='pod_logs'),

    path('send-email/', views.send_email, name='send_email'), # should be removed in prod
    path('notifications/', views.notifications, name='notifications'),
]