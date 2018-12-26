package main

const (
	// cloudProviderError is an error related to underlying infrastructure
	cloudProviderError = "cloudProviderError"
	// apiCallError is an error related to communication with k8s API server
	apiCallError = "apiCallError"
	// internalError is an error inside Cluster Autoscaler
	internalError = "internalError"
	// transientError is an error that causes us to skip a single loop, but
	// does not require any additional action.
	transientError = "transientError"
)

const (
	providerName                      = "grpc"
	errMismatchingProvider            = "Secret doesn't match with target server"
	errNodeGroupNotFound              = "Node group %s not found"
	errNodeGroupForNodeNotFound       = "NodeGroup %s not found for Node %s"
	errNodeNotFoundInNodeGroup        = "The node %s not found in node group %s"
	errMachineTypeNotFound            = "Machine type %s not found"
	errNodeGroupAlreadyExists         = "Can't create node group: %s, already exists"
	errUnableToCreateNodeGroup        = "Can't create node group: %s, reason: %v"
	errUnableToDeleteNodeGroup        = "Can't delete node group: %s, reason: %v"
	errCantDecodeNodeIDWithReason     = "Node providerID %s not conform, reason: %v"
	errCantDecodeNodeID               = "Node providerID %s not conform"
	errCantUnmarshallNodeWithReason   = "Can't unmarshall node definition:%s, reason: %v"
	errCantUnmarshallNode             = "Can't unmarshall node definition[%d] in group %s"
	errUnableToDeleteNode             = "Can't delete node: %s, because not owned by node group: %s"
	errMinSizeReached                 = "Min size reached for group: %s, nodes will not be deleted"
	errIncreaseSizeMustBePositive     = "Size increase must be positive"
	errIncreaseSizeTooLarge           = "Size increase too large, desired: %d max: %d"
	errDecreaseSizeMustBeNegative     = "Size decrease must be negative"
	errDecreaseSizeAttemptDeleteNodes = "Attempt to delete existing nodes, targetSize: %d delta: %d existingNodes: %d"
	errUnableToLaunchVM               = "Unable to launch the VM owned by node: %s, reason: %v"
	errUnableToDeleteVM               = "Unable to delete the VM owned by node: %s, reason: %v"
	errWrongSchemeInProviderID        = "Wrong scheme in providerID %s. expect AutoScaler, got: %s"
	errWrongPathInProviderID          = "Wrong path in providerID: %s. expect object, got: %s"
	errVMAlreadyCreated               = "Unable to launch VM, %s is already created"
	errUnableToMountPath              = "Unable to mount host path:%s into guest:%s for node:%s, reason: %v"
	errTempFile                       = "Can't create temp file, reason: %v"
	errCloudInitMarshallError         = "Can't marshall cloud-init, reason: %v"
	errCloudInitWriteError            = "Can't write cloud-init, reason: %v"
	errGetVMInfoFailed                = "Can't get the VM info from AutoScaler for VM: %s, reason: %v"
	errAutoScalerInfoNotFound          = "Can't find the VM info from AutoScaler for VM: %s"
	errKubeAdmJoinFailed              = "Unable to join the master kubernetes node for VM: %s, reason: %v"
	errKubeAdmJoinNotRunning          = "Could not join kubernetes master node, the VM: %s is not running"
	errStopVMFailed                   = "Could not stop VM: %s, reason: %v"
	errStartVMFailed                  = "Could not start VM: %s, reason: %v"
	errDeleteVMFailed                 = "Could not delete VM: %s, reason: %v"
	errVMNotFound                     = "Unable to find VM: %s"
	errVMStopFailed                   = "Unable to stop VM: %s before delete"
	errNodeGroupCleanupFailOnVM       = "On node group: %s, failed to delete VM: %s, reason: %v"
	errKubeCtlIgnoredError            = "kubectl got error on VM: %s, reason: %s"
	errNotImplemented                 = "Not implemented"
	errNodeIsNotReady                 = "Node %s is not ready"
	errUnableToAutoProvisionNodeGroup = "Warning can't autoprovision node group, reason: %v"
	errUnmarshallingError             = "Unable to unmarshall node: %s as json, reason: %v"
	errMarshallingError               = "Unable to marshall node: %s as json, reason: %v"
	errKubeletNotConfigured           = "Can't set provider ID in kubelet for VM: %s, %s, reason: %v"
	errVMNotProvisionnedByMe          = "The VM: %s is not provisionned by me"
	errFailedToLoadServerState        = "Failed to load server state, reason: %v"
	errFailedToSaveServerState        = "Failed to save server state, reason: %v"
)
