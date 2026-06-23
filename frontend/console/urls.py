from django.urls import path
from . import views

urlpatterns = [
    path('', views.index, name='index'),
    path('machines/', views.machines, name='machines'),
    path('machine/<str:machine_name>/', views.machine_detail, name='machine_detail'),
    path('machine/<str:machine_name>/delete/', views.machine_delete, name='machine_delete')
]