package consts

import k8sutils "github.com/kong/gateway-operator/pkg/utils/kubernetes"

const (
	// DataPlaneConditionTypeRolledOut is a condition type indicating whether or
	// not, DataPlane's rollout has been successful or not.
	DataPlaneConditionTypeRolledOut k8sutils.ConditionType = "RolledOut"
)

const (
	// DataPlaneConditionReasonRolloutAwaitingPromotion is a reason which indicates a DataPlane
	// preview has been deployed successfully and is awaiting promotion.
	// If this Reason is present and no automated rollout is disabled, user can
	// use the preview services and deployment to inspect the state of those
	// make a judgement call if the promotion should happen.
	DataPlaneConditionReasonRolloutAwaitingPromotion k8sutils.ConditionReason = "AwaitingPromotion"

	// DataPlaneConditionReasonRolloutFailed is a reason which indicates a DataPlane
	// has failed to roll out. This may be caused for example by a Deployment or
	// a Service failing to get created during a rollout.
	DataPlaneConditionReasonRolloutFailed k8sutils.ConditionReason = "Failed"

	// DataPlaneConditionReasonRolloutProgressing is a reason which indicates a DataPlane's
	// new version is being rolled out.
	DataPlaneConditionReasonRolloutProgressing k8sutils.ConditionReason = "Progressing"

	// DataPlaneConditionReasonRolloutWaitingForChange is a reason which indicates a DataPlane
	// is waiting for a change to trigger new version to be made available before promotion.
	DataPlaneConditionReasonRolloutWaitingForChange k8sutils.ConditionReason = "WaitingForChange"

	// DataPlaneConditionReasonRolloutPromotionInProgress is a reason which
	// indicates that a promotion is in progress.
	DataPlaneConditionReasonRolloutPromotionInProgress k8sutils.ConditionReason = "PromotionInProgress"

	// DataPlaneConditionReasonRolloutPromotionFailed is a reason which indicates
	// a DataPlane has failed to promote. This may be caused for example by
	// a failure in updating a live Service.
	DataPlaneConditionReasonRolloutPromotionFailed k8sutils.ConditionReason = "PromotionFailed"

	// DataPlaneConditionReasonRolloutPromotionDone is a reason which indicates that a promotion is done.
	DataPlaneConditionReasonRolloutPromotionDone k8sutils.ConditionReason = "PromotionDone"
)

const (
	// DataPlaneConditionMessageRolledOutRolloutInitialized contains the message
	// that is set for the RolledOut Condition when Reason is Progressing
	// and the DataPlane has initiated a rollout.
	DataPlaneConditionMessageRolledOutRolloutInitialized = "Rollout initialized"

	// DataPlaneConditionMessageRolledOutPreviewDeploymentNotYetReady contains the message
	// that is set for the RolledOut Condition when Reason is Progressing
	// and the operator is waiting for preview Deployment to be ready.
	DataPlaneConditionMessageRolledOutPreviewDeploymentNotYetReady = "Preview Deployment not yet ready"
)
