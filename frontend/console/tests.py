from unittest.mock import patch

from django.test import TestCase
from django.urls import reverse

from .models import Deployment


class DeleteDeploymentTests(TestCase):
    @patch("console.k3_views.k3.kube_app_api.delete_namespaced_deployment")
    def test_delete_removes_deployment_record(self, delete_namespaced_deployment):
        Deployment.objects.create(
            name="example-deployment",
            replicas_desired=1,
            service_name=None,
        )

        response = self.client.get(
            reverse(
                "delete_deployment",
                kwargs={"namespace": "default", "deployment_name": "example-deployment"},
            )
        )

        self.assertEqual(response.status_code, 302)
        self.assertFalse(Deployment.objects.filter(name="example-deployment").exists())
        delete_namespaced_deployment.assert_called_once_with(
            name="example-deployment", namespace="default"
        )
