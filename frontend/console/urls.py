from django.urls import path
from . import views

urlpatterns = [
    path('', views.index, name='index'),
    path('login/', views.login_view, name='login'),
    path('passkeys-mgmt', views.passkeys_mgmt, name='passkeys_mgmt'),
    path('logout/', views.logout_view, name='logout'),
    path('machines/', views.machines, name='machines'),
    path('machine/<str:machine_name>/', views.machine_detail, name='machine_detail'),
    path('machine/<str:machine_name>/delete/', views.machine_delete, name='machine_delete'),
    path("machine/<str:machine_name>/vms/create", views.create_vm, name="create_vm"),
    path("machine/<str:machine_name>/vms/<str:vm_name>", views.vm_detail, name="vm_detail"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/start", views.vm_start, name="vm_start"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/stop", views.vm_stop, name="vm_stop"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/restart", views.vm_restart, name="vm_restart"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/delete", views.vm_delete, name="vm_delete"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/connect", views.vm_connect, name="vm_connect"),
    path("machine/<str:machine_name>/vms/<str:vm_name>/<str:session_id>/disconnect", views.vm_disconnect, name="vm_disconnect"),
    path("machine/<str:machine_name>/containers/<str:container_id>", views.container_detail, name="container_detail"),
    path("machine/<str:machine_name>/containers/<str:container_id>/start", views.container_start, name="container_start"),
    path("machine/<str:machine_name>/containers/<str:container_id>/stop", views.container_stop, name="container_stop"),
    path("machine/<str:machine_name>/containers/<str:container_id>/remove", views.container_remove, name="container_remove"),
    path("machine/<str:machine_name>/containers/<str:container_id>/stream_logs", views.stream_container_logs, name="stream_container_logs"),
    # path("vms/", views.vms, name="vms")
    path('send-email/', views.send_email, name='send_email'), # should be removed in prod
    path('notifications/', views.notifications, name='notifications'),
    path('k3/<str:namespace>/<str:pod_name>/', views.pod_detail, name='pod_detail'),
]