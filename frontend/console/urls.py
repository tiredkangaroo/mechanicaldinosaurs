from django.urls import path
from . import views
from . import auth_views
from . import machine_views
from . import vm_views
from . import k3_views
from . import automation_views
from . import tunnels_views
from . import shell_views

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
    path("machine/<str:machine_name>/download-iso", vm_views.download_iso, name="download_iso"),

    path("machine/<str:machine_name>/shell", shell_views.shell, name="shell"),
    path("machine/<str:machine_name>/shell/<str:session_id>/kill", shell_views.kill_shell_session, name="kill_shell_session"),

    path('k3/deploy/', k3_views.deploy, name='deploy'),
    path('k3/deployments/<str:namespace>/<str:deployment_name>/', k3_views.deployment_detail, name='deployment_detail'),
    path('k3/deployments/<str:namespace>/<str:deployment_name>/start', k3_views.start_deployment, name='start_deployment'),
    path('k3/deployments/<str:namespace>/<str:deployment_name>/stop', k3_views.stop_deployment, name='stop_deployment'),
    path('k3/deployments/<str:namespace>/<str:deployment_name>/delete', k3_views.delete_deployment, name='delete_deployment'), 
    path('k3/pod/<str:namespace>/<str:pod_name>/logs', k3_views.pod_logs, name='pod_logs'),

    path('automations/create/', automation_views.create_automation, name='create_automation'),
    path('automations/<str:automation_id>/', automation_views.automation_detail, name='automation_detail'),
    path('automations/<str:automation_id>/enable/', automation_views.enable_automation, name='enable_automation'),
    path('automations/<str:automation_id>/disable/', automation_views.disable_automation, name='disable_automation'),
    path('automations/<str:automation_id>/delete/', automation_views.delete_automation, name='delete_automation'),
    path('automations/<str:automation_id>/clear-error-logs', automation_views.clear_automation_error_logs, name='clear_automation_error_logs'),

    path('tunnels/', tunnels_views.tunnels, name='tunnels'),
    path('machine/<str:machine_name>/tunnels/add-tunnel/', tunnels_views.add_tunnel, name='add_tunnel'),
    path('machine/<str:machine_name>/tunnels/<str:tunnel_id>/remove', tunnels_views.remove_tunnel, name='remove_tunnel'),
]