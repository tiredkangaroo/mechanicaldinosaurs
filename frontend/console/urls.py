from django.urls import path
from . import views

urlpatterns = [
    path('', views.index, name='index'),
    path('machines/', views.machines, name='machines'),
    path('machine/<str:machine_name>/', views.machine_detail, name='machine_detail'),
    path('machine/<str:machine_name>/delete/', views.machine_delete, name='machine_delete'),
    path("machine/<str:machine_name>/vms/create", views.create_vm, name="create_vm"),
    path("machine/<str:machine_name>/vms/<str:vm_name>", views.vm_detail, name="vm_detail"),
    # path("machine/<str:machine_name>/vms/<str:vm_name>/delete", views.vm_delete, name="vm_delete"),
    # path("vms/", views.vms, name="vms")
]