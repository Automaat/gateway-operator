package test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	operatorv1alpha1 "github.com/kong/gateway-operator/apis/v1alpha1"
	operatorv1beta1 "github.com/kong/gateway-operator/apis/v1beta1"
	"github.com/kong/gateway-operator/controllers/controlplane"
	gwtypes "github.com/kong/gateway-operator/internal/types"
	"github.com/kong/gateway-operator/pkg/clientset"
	"github.com/kong/gateway-operator/pkg/consts"
	gatewayutils "github.com/kong/gateway-operator/pkg/utils/gateway"
	k8sutils "github.com/kong/gateway-operator/pkg/utils/kubernetes"
)

// controlPlanePredicate is a helper function for tests that returns a function
// that can be used to check if a ControlPlane has a certain state.
func controlPlanePredicate(
	t *testing.T,
	ctx context.Context,
	controlplaneName types.NamespacedName,
	predicate func(controlplane *operatorv1alpha1.ControlPlane) bool,
	operatorClient *clientset.Clientset,
) func() bool {
	controlplaneClient := operatorClient.ApisV1alpha1().ControlPlanes(controlplaneName.Namespace)
	return func() bool {
		controlplane, err := controlplaneClient.Get(ctx, controlplaneName.Name, metav1.GetOptions{})
		require.NoError(t, err)
		return predicate(controlplane)
	}
}

// DataPlanePredicate is a helper function for tests that returns a function
// that can be used to check if a DataPlane has a certain state.
func DataPlanePredicate(
	t *testing.T,
	ctx context.Context,
	dataplaneName types.NamespacedName,
	predicate func(dataplane *operatorv1beta1.DataPlane) bool,
	operatorClient *clientset.Clientset,
) func() bool {
	dataPlaneClient := operatorClient.ApisV1beta1().DataPlanes(dataplaneName.Namespace)
	return func() bool {
		dataplane, err := dataPlaneClient.Get(ctx, dataplaneName.Name, metav1.GetOptions{})
		require.NoError(t, err)
		return predicate(dataplane)
	}
}

// HPAPredicate is a helper function for tests that returns a function
// that can be used to check if an HPA has a certain state.
func HPAPredicate(
	t *testing.T,
	ctx context.Context,
	hpaName types.NamespacedName,
	predicate func(hpa *autoscalingv2.HorizontalPodAutoscaler) bool,
	client client.Client,
) func() bool {
	return func() bool {
		var hpa autoscalingv2.HorizontalPodAutoscaler
		require.NoError(t, client.Get(ctx, hpaName, &hpa))
		return predicate(&hpa)
	}
}

// ControlPlaneIsScheduled is a helper function for tests that returns a function
// that can be used to check if a ControlPlane was scheduled.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func ControlPlaneIsScheduled(t *testing.T, ctx context.Context, controlPlane types.NamespacedName, operatorClient *clientset.Clientset) func() bool {
	return controlPlanePredicate(t, ctx, controlPlane, func(c *operatorv1alpha1.ControlPlane) bool {
		for _, condition := range c.Status.Conditions {
			if condition.Type == string(controlplane.ConditionTypeProvisioned) {
				return true
			}
		}
		return false
	}, operatorClient)
}

// DataPlaneIsReady is a helper function for tests that returns a function
// that can be used to check if a DataPlane is ready.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func DataPlaneIsReady(t *testing.T, ctx context.Context, dataplane types.NamespacedName, operatorClient *clientset.Clientset) func() bool {
	return DataPlanePredicate(t, ctx, dataplane, func(c *operatorv1beta1.DataPlane) bool {
		for _, condition := range c.Status.Conditions {
			if condition.Type == string(k8sutils.ReadyType) && condition.Status == metav1.ConditionTrue {
				return true
			}
		}
		return false
	}, operatorClient)
}

// ControlPlaneDetectedNoDataPlane is a helper function for tests that returns a function
// that can be used to check if a ControlPlane detected unset dataplane.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func ControlPlaneDetectedNoDataPlane(t *testing.T, ctx context.Context, controlPlane types.NamespacedName, clients K8sClients) func() bool {
	return controlPlanePredicate(t, ctx, controlPlane, func(c *operatorv1alpha1.ControlPlane) bool {
		for _, condition := range c.Status.Conditions {
			if condition.Type == string(controlplane.ConditionTypeProvisioned) &&
				condition.Status == metav1.ConditionFalse &&
				condition.Reason == string(controlplane.ConditionReasonNoDataPlane) {
				return true
			}
		}
		return false
	}, clients.OperatorClient)
}

// ControlPlaneIsProvisioned is a helper function for tests that returns a function
// that can be used to check if a ControlPlane was provisioned.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func ControlPlaneIsProvisioned(t *testing.T, ctx context.Context, controlPlane types.NamespacedName, clients K8sClients) func() bool {
	return controlPlanePredicate(t, ctx, controlPlane, func(c *operatorv1alpha1.ControlPlane) bool {
		for _, condition := range c.Status.Conditions {
			if condition.Type == string(controlplane.ConditionTypeProvisioned) &&
				condition.Status == metav1.ConditionTrue {
				return true
			}
		}
		return false
	}, clients.OperatorClient)
}

// ControlPlaneIsNotReady is a helper function for tests. It returns a function
// that can be used to check if a ControlPlane is marked as not Ready.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func ControlPlaneIsNotReady(t *testing.T, ctx context.Context, controlplane types.NamespacedName, clients K8sClients) func() bool {
	return controlPlanePredicate(t, ctx, controlplane, func(c *operatorv1alpha1.ControlPlane) bool {
		for _, condition := range c.Status.Conditions {
			if condition.Type == string(k8sutils.ReadyType) &&
				condition.Status == metav1.ConditionFalse {
				return true
			}
		}
		return false
	}, clients.OperatorClient)
}

// ControlPlaneIsReady is a helper function for tests. It returns a function
// that can be used to check if a ControlPlane is marked as Ready.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func ControlPlaneIsReady(t *testing.T, ctx context.Context, controlplane types.NamespacedName, clients K8sClients) func() bool {
	return controlPlanePredicate(t, ctx, controlplane, func(c *operatorv1alpha1.ControlPlane) bool {
		for _, condition := range c.Status.Conditions {
			if condition.Type == string(k8sutils.ReadyType) &&
				condition.Status == metav1.ConditionTrue {
				return true
			}
		}
		return false
	}, clients.OperatorClient)
}

// ControlPlaneHasActiveDeployment is a helper function for tests that returns a function
// that can be used to check if a ControlPlane has an active deployment.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func ControlPlaneHasActiveDeployment(t *testing.T, ctx context.Context, controlplaneName types.NamespacedName, clients K8sClients) func() bool {
	return controlPlanePredicate(t, ctx, controlplaneName, func(controlplane *operatorv1alpha1.ControlPlane) bool {
		deployments := MustListControlPlaneDeployments(t, ctx, controlplane, clients)
		return len(deployments) == 1 &&
			*deployments[0].Spec.Replicas > 0 &&
			deployments[0].Status.AvailableReplicas == *deployments[0].Spec.Replicas
	}, clients.OperatorClient)
}

// ControlPlaneHasClusterRole is a helper function for tests that returns a function
// that can be used to check if a ControlPlane has a ClusterRole.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func ControlPlaneHasClusterRole(t *testing.T, ctx context.Context, controlplane *operatorv1alpha1.ControlPlane, clients K8sClients) func() bool {
	return func() bool {
		clusterRoles := MustListControlPlaneClusterRoles(t, ctx, controlplane, clients)
		t.Logf("%d clusterroles", len(clusterRoles))
		return len(clusterRoles) > 0
	}
}

// ControlPlaneHasClusterRoleBinding is a helper function for tests that returns a function
// that can be used to check if a ControlPlane has a ClusterRoleBinding.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func ControlPlaneHasClusterRoleBinding(t *testing.T, ctx context.Context, controlplane *operatorv1alpha1.ControlPlane, clients K8sClients) func() bool {
	return func() bool {
		clusterRoleBindings := MustListControlPlaneClusterRoleBindings(t, ctx, controlplane, clients)
		t.Logf("%d clusterrolebindings", len(clusterRoleBindings))
		return len(clusterRoleBindings) > 0
	}
}

func ControlPlaneHasNReadyPods(t *testing.T, ctx context.Context, controlplaneName types.NamespacedName, clients K8sClients, n int) func() bool {
	return controlPlanePredicate(t, ctx, controlplaneName, func(controlplane *operatorv1alpha1.ControlPlane) bool {
		deployments := MustListControlPlaneDeployments(t, ctx, controlplane, clients)
		return len(deployments) == 1 &&
			*deployments[0].Spec.Replicas == int32(n) &&
			deployments[0].Status.AvailableReplicas == *deployments[0].Spec.Replicas
	}, clients.OperatorClient)
}

// DataPlaneHasActiveDeployment is a helper function for tests that returns a function
// that can be used to check if a DataPlane has an active deployment (that is,
// a Deployment that has at least 1 Replica and that all Replicas as marked as Available).
// Should be used in conjunction with require.Eventually or assert.Eventually.
func DataPlaneHasActiveDeployment(
	t *testing.T,
	ctx context.Context,
	dataplaneNN types.NamespacedName,
	ret *appsv1.Deployment,
	matchingLabels client.MatchingLabels,
	clients K8sClients,
) func() bool {
	return DataPlanePredicate(t, ctx, dataplaneNN, func(dataplane *operatorv1beta1.DataPlane) bool {
		deployments := MustListDataPlaneDeployments(t, ctx, dataplane, clients, matchingLabels)
		if len(deployments) == 1 &&
			deployments[0].Status.AvailableReplicas == *deployments[0].Spec.Replicas {
			if ret != nil {
				*ret = deployments[0]
			}
			return true
		}
		return false
	}, clients.OperatorClient)
}

// DataPlaneHasHPA is a helper function for tests that returns a function
// that can be used to check if a DataPlane has an active HPA.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func DataPlaneHasHPA(
	t *testing.T,
	ctx context.Context,
	dataplane *operatorv1beta1.DataPlane,
	ret *autoscalingv2.HorizontalPodAutoscaler,
	clients K8sClients,
) func() bool {
	dataplaneName := client.ObjectKeyFromObject(dataplane)
	const dataplaneDeploymentAppLabel = "app"

	return DataPlanePredicate(t, ctx, dataplaneName, func(dataplane *operatorv1beta1.DataPlane) bool {
		deployments := MustListDataPlaneDeployments(t, ctx, dataplane, clients, client.MatchingLabels{
			dataplaneDeploymentAppLabel:          dataplane.Name,
			consts.GatewayOperatorManagedByLabel: consts.DataPlaneManagedLabelValue,
			consts.DataPlaneDeploymentStateLabel: consts.DataPlaneStateLabelValueLive, // Only live Deployment has an HPA.
		})
		if len(deployments) != 1 {
			return false
		}

		hpas := MustListDataPlaneHPAs(t, ctx, dataplane, clients, client.MatchingLabels{
			dataplaneDeploymentAppLabel:          dataplane.Name,
			consts.GatewayOperatorManagedByLabel: consts.DataPlaneManagedLabelValue,
		})
		if len(hpas) != 1 {
			return false
		}

		hpa := hpas[0]
		if hpa.Spec.ScaleTargetRef.Name != deployments[0].Name {
			return false
		}

		if ret != nil {
			*ret = hpa
		}

		return true
	}, clients.OperatorClient)
}

// DataPlaneHasDeployment is a helper function for tests that returns a function
// that can be used to check if a DataPlane has a Deployment.
// Optionally the caller can provide a list of assertions that will be checked
// against the found Deployment.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func DataPlaneHasDeployment(
	t *testing.T,
	ctx context.Context,
	dataplaneName types.NamespacedName,
	ret *appsv1.Deployment,
	clients K8sClients,
	matchingLabels client.MatchingLabels,
	asserts ...func(appsv1.Deployment) bool,
) func() bool {
	return DataPlanePredicate(t, ctx, dataplaneName, func(dataplane *operatorv1beta1.DataPlane) bool {
		deployments := MustListDataPlaneDeployments(t, ctx, dataplane, clients, matchingLabels)
		if len(deployments) != 1 {
			return false
		}
		deployment := deployments[0]
		for _, a := range asserts {
			if !a(deployment) {
				return false
			}
		}
		if ret != nil {
			*ret = deployment
		}
		return true
	}, clients.OperatorClient)
}

func DataPlaneHasNReadyPods(t *testing.T, ctx context.Context, dataplaneName types.NamespacedName, clients K8sClients, n int) func() bool {
	return DataPlanePredicate(t, ctx, dataplaneName, func(dataplane *operatorv1beta1.DataPlane) bool {
		deployments := MustListDataPlaneDeployments(t, ctx, dataplane, clients, client.MatchingLabels{
			consts.GatewayOperatorManagedByLabel: consts.DataPlaneManagedLabelValue,
		})
		return len(deployments) == 1 &&
			*deployments[0].Spec.Replicas == int32(n) &&
			deployments[0].Status.AvailableReplicas == *deployments[0].Spec.Replicas
	}, clients.OperatorClient)
}

// DataPlaneHasService is a helper function for tests that returns a function
// that can be used to check if a DataPlane has a service created.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func DataPlaneHasService(t *testing.T, ctx context.Context, dataplaneName types.NamespacedName, clients K8sClients, matchingLabels client.MatchingLabels) func() bool {
	return DataPlanePredicate(t, ctx, dataplaneName, func(dataplane *operatorv1beta1.DataPlane) bool {
		services := MustListDataPlaneServices(t, ctx, dataplane, clients.MgrClient, matchingLabels)
		return len(services) == 1
	}, clients.OperatorClient)
}

// DataPlaneHasActiveService is a helper function for tests that returns a function
// that can be used to check if a DataPlane has an active proxy service.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func DataPlaneHasActiveService(t *testing.T, ctx context.Context, dataplaneName types.NamespacedName, ret *corev1.Service, clients K8sClients, matchingLabels client.MatchingLabels) func() bool {
	return DataPlanePredicate(t, ctx, dataplaneName, func(dataplane *operatorv1beta1.DataPlane) bool {
		services := MustListDataPlaneServices(t, ctx, dataplane, clients.MgrClient, matchingLabels)
		if len(services) == 1 {
			if ret != nil {
				*ret = services[0]
			}
			return true
		}
		return false
	}, clients.OperatorClient)
}

// DataPlaneServiceHasNActiveEndpoints is a helper function for tests that returns a function
// that can be used to check if a Service has active endpoints.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func DataPlaneServiceHasNActiveEndpoints(t *testing.T, ctx context.Context, serviceName types.NamespacedName, clients K8sClients, n int) func() bool {
	return func() bool {
		endpointSlices := MustListServiceEndpointSlices(
			t,
			ctx,
			serviceName,
			clients.MgrClient,
		)
		if len(endpointSlices) != 1 {
			return false
		}
		return len(endpointSlices[0].Endpoints) == n
	}
}

// DataPlaneHasServiceAndAddressesInStatus is a helper function for tests that returns
// a function that can be used to check if a DataPlane has:
// - a backing service name in its .Service status field
// - a list of addreses of its backing service in its .Addresses status field
// Should be used in conjunction with require.Eventually or assert.Eventually.
func DataPlaneHasServiceAndAddressesInStatus(t *testing.T, ctx context.Context, dataplaneName types.NamespacedName, clients K8sClients) func() bool {
	return DataPlanePredicate(t, ctx, dataplaneName, func(dataplane *operatorv1beta1.DataPlane) bool {
		services := MustListDataPlaneServices(t, ctx, dataplane, clients.MgrClient, client.MatchingLabels{
			consts.GatewayOperatorManagedByLabel: consts.DataPlaneManagedLabelValue,
			consts.DataPlaneServiceTypeLabel:     string(consts.DataPlaneIngressServiceLabelValue),
		})
		if len(services) != 1 {
			return false
		}
		service := services[0]
		if dataplane.Status.Service != service.Name {
			t.Logf("DataPlane %q: found %q as backing service, wanted %q",
				dataplane.Name, dataplane.Status.Service, service.Name,
			)
			return false
		}

		var wanted []string
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			if ingress.IP != "" {
				wanted = append(wanted, ingress.IP)
			}
			if ingress.Hostname != "" {
				wanted = append(wanted, ingress.Hostname)
			}
		}
		wanted = append(wanted, service.Spec.ClusterIPs...)

		var addresses []string
		for _, addr := range dataplane.Status.Addresses {
			addresses = append(addresses, addr.Value)
		}

		if len(addresses) != len(wanted) {
			t.Logf("DataPlane %q: found %d addresses %v, wanted %d %v",
				dataplane.Name, len(addresses), addresses, len(wanted), wanted,
			)
			return false
		}

		if !cmp.Equal(addresses, wanted) {
			t.Logf("DataPlane %q: found addresses %v, wanted %v",
				dataplane.Name, addresses, wanted,
			)
			return false
		}

		return true
	}, clients.OperatorClient)
}

// DataPlaneUpdateEventually is a helper function for tests that returns a function
// that can be used to update the DataPlane.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func DataPlaneUpdateEventually(t *testing.T, ctx context.Context, dataplaneNN types.NamespacedName, clients K8sClients, updateFunc func(*operatorv1beta1.DataPlane)) func() bool {
	return func() bool {
		cl := clients.OperatorClient.ApisV1beta1().DataPlanes(dataplaneNN.Namespace)
		dp, err := cl.Get(ctx, dataplaneNN.Name, metav1.GetOptions{})
		if err != nil {
			t.Logf("error getting dataplane: %v", err)
			return false
		}

		updateFunc(dp)

		_, err = cl.Update(ctx, dp, metav1.UpdateOptions{})
		if err != nil {
			t.Logf("error updating dataplane: %v", err)
			return false
		}
		return true
	}
}

func DataPlaneHasServiceSecret(t *testing.T, ctx context.Context, dpNN, usingSvc types.NamespacedName, ret *corev1.Secret, clients K8sClients) func() bool {
	return DataPlanePredicate(t, ctx, dpNN, func(dp *operatorv1beta1.DataPlane) bool {
		secrets, err := k8sutils.ListSecretsForOwner(ctx, clients.MgrClient, dp.GetUID(), client.MatchingLabels{
			consts.GatewayOperatorManagedByLabel: consts.DataPlaneManagedLabelValue,
			consts.ServiceSecretLabel:            usingSvc.Name,
		})
		if err != nil {
			t.Logf("error listing secrets: %v", err)
			return false
		}
		if len(secrets) == 1 {
			*ret = secrets[0]
			return true
		}
		return false
	}, clients.OperatorClient)
}

// GatewayClassIsAccepted is a helper function for tests that returns a function
// that can be used to check if a GatewayClass is accepted.
// Should be used in conjunction with require.Eventually or assert.Eventually.
func GatewayClassIsAccepted(t *testing.T, ctx context.Context, gatewayClassName string, clients K8sClients) func() bool {
	gatewayClasses := clients.GatewayClient.GatewayV1().GatewayClasses()

	return func() bool {
		gwc, err := gatewayClasses.Get(context.Background(), gatewayClassName, metav1.GetOptions{})
		if err != nil {
			return false
		}
		for _, cond := range gwc.Status.Conditions {
			if cond.Reason == string(gatewayv1.GatewayClassConditionStatusAccepted) {
				if cond.ObservedGeneration == gwc.Generation {
					return true
				}
			}
		}
		return false
	}
}

// GatewayNotExist is a helper function for tests that returns a function
// to check a if gateway(with specified namespace and name) does not exist.
//
//	Should be used in conjunction with require.Eventually or assert.Eventually.
func GatewayNotExist(t *testing.T, ctx context.Context, gatewayNSN types.NamespacedName, clients K8sClients) func() bool {
	return func() bool {
		gateways := clients.GatewayClient.GatewayV1().Gateways(gatewayNSN.Namespace)
		_, err := gateways.Get(ctx, gatewayNSN.Name, metav1.GetOptions{})
		if err != nil {
			return errors.IsNotFound(err)
		}
		return false
	}
}

func GatewayIsScheduled(t *testing.T, ctx context.Context, gatewayNSN types.NamespacedName, clients K8sClients) func() bool {
	return func() bool {
		return gatewayutils.IsScheduled(MustGetGateway(t, ctx, gatewayNSN, clients))
	}
}

func GatewayIsProgrammed(t *testing.T, ctx context.Context, gatewayNSN types.NamespacedName, clients K8sClients) func() bool {
	return func() bool {
		return gatewayutils.IsProgrammed(MustGetGateway(t, ctx, gatewayNSN, clients))
	}
}

func GatewayListenersAreProgrammed(t *testing.T, ctx context.Context, gatewayNSN types.NamespacedName, clients K8sClients) func() bool {
	return func() bool {
		return gatewayutils.AreListenersProgrammed(MustGetGateway(t, ctx, gatewayNSN, clients))
	}
}

func GatewayDataPlaneIsReady(t *testing.T, ctx context.Context, gateway *gwtypes.Gateway, clients K8sClients) func() bool {
	return func() bool {
		dataplanes := MustListDataPlanesForGateway(t, ctx, gateway, clients)

		if len(dataplanes) == 1 {
			// if the dataplane DeletionTimestamp is set, the dataplane deletion has been requested.
			// Hence we cannot consider it as a valid dataplane that's ready.
			if dataplanes[0].DeletionTimestamp != nil {
				return false
			}
			for _, condition := range dataplanes[0].Status.Conditions {
				if condition.Type == string(k8sutils.ReadyType) &&
					condition.Status == metav1.ConditionTrue {
					return true
				}
			}
		}
		return false
	}
}

func GatewayControlPlaneIsProvisioned(t *testing.T, ctx context.Context, gateway *gwtypes.Gateway, clients K8sClients) func() bool {
	return func() bool {
		controlPlanes := MustListControlPlanesForGateway(t, ctx, gateway, clients)

		if len(controlPlanes) == 1 {
			// if the controlplane DeletionTimestamp is set, the controlplane deletion has been requested.
			// Hence we cannot consider it as a provisioned valid controlplane.
			if controlPlanes[0].DeletionTimestamp != nil {
				return false
			}
			for _, condition := range controlPlanes[0].Status.Conditions {
				if condition.Type == string(controlplane.ConditionTypeProvisioned) &&
					condition.Status == metav1.ConditionTrue {
					return true
				}
			}
		}
		return false
	}
}

// GatewayNetworkPoliciesExist is a helper function for tests that returns a function
// that can be used to check if a Gateway owns a networkpolicy.
// Should be used in conjunction with require.Eventually or assert.Eventually.
// Gateway object argument does need to exist in the cluster, thus, the function
// may be used with Not after the gateway has been deleted, to verify that
// the networkpolicy has been deleted too.
func GatewayNetworkPoliciesExist(t *testing.T, ctx context.Context, gateway *gwtypes.Gateway, clients K8sClients) func() bool {
	return func() bool {
		networkpolicies, err := gatewayutils.ListNetworkPoliciesForGateway(ctx, clients.MgrClient, gateway)
		if err != nil {
			return false
		}
		return len(networkpolicies) > 0
	}
}

type ingressRuleT interface {
	netv1.NetworkPolicyIngressRule | netv1.NetworkPolicyEgressRule
}

// GatewayNetworkPolicyForGatewayContainsRules is a helper function for tets that
// returns a function that can be used to check if exactly 1 NetworkPolicy exist
// for Gateway and if it contains all the provided rules.
func GatewayNetworkPolicyForGatewayContainsRules[T ingressRuleT](t *testing.T, ctx context.Context, gateway *gwtypes.Gateway, clients K8sClients, rules ...T) func() bool {
	return func() bool {
		networkpolicies, err := gatewayutils.ListNetworkPoliciesForGateway(ctx, clients.MgrClient, gateway)
		if err != nil {
			return false
		}

		if len(networkpolicies) != 1 {
			return false
		}

		netpol := networkpolicies[0]
		for _, rule := range rules {
			switch r := any(rule).(type) {
			case netv1.NetworkPolicyIngressRule:
				if !networkPolicyRuleSliceContainsRule(netpol.Spec.Ingress, r) {
					return false
				}
			case netv1.NetworkPolicyEgressRule:
				if !networkPolicyRuleSliceContainsRule(netpol.Spec.Egress, r) {
					return false
				}
			default:
				t.Logf("NetworkPolicy rule has an unknown type %T", rule)
			}
		}
		return true
	}
}

func networkPolicyRuleSliceContainsRule[T ingressRuleT](rules []T, rule T) bool {
	for _, r := range rules {
		if cmp.Equal(r, rule) {
			return true
		}
	}

	return false
}

func GatewayIPAddressExist(t *testing.T, ctx context.Context, gatewayNSN types.NamespacedName, clients K8sClients) func() bool {
	return func() bool {
		gateway := MustGetGateway(t, ctx, gatewayNSN, clients)
		if len(gateway.Status.Addresses) > 0 && *gateway.Status.Addresses[0].Type == gatewayv1.IPAddressType {
			return true
		}
		return false
	}
}

func GetResponseBodyContains(t *testing.T, ctx context.Context, clients K8sClients, httpc http.Client, url string, responseContains string) func() bool {
	return func() bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		require.NoError(t, err)

		resp, err := httpc.Do(req)
		if err != nil {
			return false
		}

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		return strings.Contains(string(body), responseContains)
	}
}

// Not is a helper function for tests that returns a negation of a predicate.
func Not(predicate func() bool) func() bool {
	return func() bool {
		return !predicate()
	}
}
