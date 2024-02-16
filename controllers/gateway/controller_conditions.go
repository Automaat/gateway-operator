package gateway

import (
	k8sutils "github.com/kong/gateway-operator/pkg/utils/kubernetes"
)

// -----------------------------------------------------------------------------
// Gateway - Status Condition Types
// -----------------------------------------------------------------------------

const (
	// GatewayServiceType the Gateway service condition type
	GatewayServiceType k8sutils.ConditionType = "GatewayService"

	// ControlPlaneReadyType the ControlPlane is deployed and Ready
	ControlPlaneReadyType k8sutils.ConditionType = "ControlPlaneReady"

	// DataPlaneReadyType the DataPlane is deployed and Ready
	DataPlaneReadyType k8sutils.ConditionType = "DataPlaneReady"
)

// -----------------------------------------------------------------------------
// Gateway - Status Condition Reasons
// -----------------------------------------------------------------------------

const (
	// GatewayServiceErrorReason the Gateway Service is not properly configured
	GatewayServiceErrorReason k8sutils.ConditionReason = "GatewayServiceError"
)
