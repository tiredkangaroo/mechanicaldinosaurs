from django.urls import path
from . import views

urlpatterns = [
    path('', views.index, name='index'),
    path('machines/', views.list_machines, name='list_machines'),
]