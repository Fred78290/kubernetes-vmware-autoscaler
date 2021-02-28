package constantes

const (
	// CloudProviderError is an error related to underlying infrastructure
	CloudProviderError = "cloudProviderError"

	// APICallError is an error related to communication with k8s API server
	APICallError = "apiCallError"

	// InternalError is an error inside Cluster Autoscaler
	InternalError = "internalError"

	// TransientError is an error that causes us to skip a single loop, but
	// does not require any additional action.
	TransientError = "transientError"
)

const (
	// ProviderName string
	ProviderName = "grpc"

	// ErrMismatchingProvider error msg
	ErrMismatchingProvider = "Secret doesn't match with target server"

	// ErrNodeGroupNotFound error msg
	ErrNodeGroupNotFound = "Node group %s not found"

	// ErrNodeGroupForNodeNotFound error msg
	ErrNodeGroupForNodeNotFound = "NodeGroup %s not found for Node %s"

	// ErrNodeNotFoundInNodeGroup error msg
	ErrNodeNotFoundInNodeGroup = "The node %s not found in node group %s"

	// ErrMachineTypeNotFound error msg
	ErrMachineTypeNotFound = "Machine type %s not found"

	// ErrNodeGroupAlreadyExists error msg
	ErrNodeGroupAlreadyExists = "Can't create node group: %s, already exists"

	// ErrUnableToCreateNodeGroup error msg
	ErrUnableToCreateNodeGroup = "Can't create node group: %s, reason: %v"

	// ErrUnableToDeleteNodeGroup error msg
	ErrUnableToDeleteNodeGroup = "Can't delete node group: %s, reason: %v"

	// ErrCantDecodeNodeIDWithReason error msg
	ErrCantDecodeNodeIDWithReason = "Node providerID %s not conform, reason: %v"

	// ErrCantDecodeNodeID error msg
	ErrCantDecodeNodeID = "Node providerID %s not conform"

	// ErrCantUnmarshallNodeWithReason error msg
	ErrCantUnmarshallNodeWithReason = "Can't unmarshall node definition:%s, reason: %v"

	// ErrCantUnmarshallNode error msg
	ErrCantUnmarshallNode = "Can't unmarshall node definition[%d] in group %s"

	// ErrUnableToDeleteNode error msg
	ErrUnableToDeleteNode = "Can't delete node: %s, because not owned by node group: %s"

	// ErrMinSizeReached error msg
	ErrMinSizeReached = "Min size reached for group: %s, nodes will not be deleted"

	// ErrIncreaseSizeMustBePositive error msg
	ErrIncreaseSizeMustBePositive = "Size increase must be positive"

	// ErrIncreaseSizeTooLarge error msg
	ErrIncreaseSizeTooLarge = "Size increase too large, desired: %d max: %d"

	// ErrDecreaseSizeMustBeNegative error msg
	ErrDecreaseSizeMustBeNegative = "Size decrease must be negative"

	// ErrDecreaseSizeAttemptDeleteNodes error msg
	ErrDecreaseSizeAttemptDeleteNodes = "Attempt to delete existing nodes, targetSize: %d delta: %d existingNodes: %d"

	// ErrUnableToLaunchVM error msg
	ErrUnableToLaunchVM = "Unable to launch the VM owned by node: %s, reason: %v"

	// ErrUnableToDeleteVM error msg
	ErrUnableToDeleteVM = "Unable to delete the VM owned by node: %s, reason: %v"

	// ErrWrongSchemeInProviderID error msg
	ErrWrongSchemeInProviderID = "Wrong scheme in providerID %s. expect %s, got: %s"

	// ErrWrongPathInProviderID error msg
	ErrWrongPathInProviderID = "Wrong path in providerID: %s. expect object, got: %s"

	// ErrVMAlreadyCreated error msg
	ErrVMAlreadyCreated = "Unable to launch VM, %s is already created"

	// ErrUnableToMountPath error msg
	ErrUnableToMountPath = "Unable to mount host path:%s into guest:%s for node:%s, reason: %v"

	// ErrTempFile error msg
	ErrTempFile = "Can't create temp file, reason: %v"

	// ErrCloudInitMarshallError error msg
	ErrCloudInitMarshallError = "Can't marshall cloud-init, reason: %v"

	// ErrCloudInitWriteError error msg
	ErrCloudInitWriteError = "Can't write cloud-init, reason: %v"

	// ErrGetVMInfoFailed error msg
	ErrGetVMInfoFailed = "Can't get the VM info from AutoScaler for VM: %s, reason: %v"

	// ErrAutoScalerInfoNotFound error msg
	ErrAutoScalerInfoNotFound = "Can't find the VM info from AutoScaler for VM: %s"

	// ErrKubeAdmJoinFailed error msg
	ErrKubeAdmJoinFailed = "Unable to join the master kubernetes node for VM: %s, reason: %v"

	// ErrKubeAdmJoinNotRunning error msg
	ErrKubeAdmJoinNotRunning = "Could not join kubernetes master node, the VM: %s is not running"

	// ErrStopVMFailed error msg
	ErrStopVMFailed = "Could not stop VM: %s, reason: %v"

	// ErrStartVMFailed error msg
	ErrStartVMFailed = "Could not start VM: %s, reason: %v"

	// ErrDeleteVMFailed error msg
	ErrDeleteVMFailed = "Could not delete VM: %s, reason: %v"

	// ErrVMNotFound error msg
	ErrVMNotFound = "Unable to find VM: %s"

	// ErrVMStopFailed error msg
	ErrVMStopFailed = "Unable to stop VM: %s before delete"

	// ErrNodeGroupCleanupFailOnVM error msg
	ErrNodeGroupCleanupFailOnVM = "On node group: %s, failed to delete VM: %s, reason: %v"

	// ErrKubeCtlIgnoredError error msg
	ErrKubeCtlIgnoredError = "kubectl got error on VM: %s, reason: %s"

	// ErrKubeCtlReturnError error msg
	ErrKubeCtlReturnError = "kubectl got error on VM: %s, %s, reason: %s"

	// ErrNotImplemented error msg
	ErrNotImplemented = "Not implemented"

	// ErrNodeIsNotReady error msg
	ErrNodeIsNotReady = "Node %s is not ready"

	// ErrUnableToAutoProvisionNodeGroup error msg
	ErrUnableToAutoProvisionNodeGroup = "Warning can't autoprovision node group, reason: %v"

	// ErrUnmarshallingError error msg
	ErrUnmarshallingError = "Unable to unmarshall node: %s as json, reason: %v"

	// ErrMarshallingError error msg
	ErrMarshallingError = "Unable to marshall node: %s as json, reason: %v"

	// ErrKubeletNotConfigured error msg
	ErrKubeletNotConfigured = "Can't set provider ID in kubelet for VM: %s, %s, reason: %v"

	// ErrVMNotProvisionnedByMe error msg
	ErrVMNotProvisionnedByMe = "The VM: %s is not provisionned by me"

	// ErrFailedToLoadServerState error msg
	ErrFailedToLoadServerState = "Failed to load server state, reason: %v"

	// ErrFailedToSaveServerState error msg
	ErrFailedToSaveServerState = "Failed to save server state, reason: %v"

	// ErrRsyncError error msg
	ErrRsyncError = "Can't rsync folder for VM: %s, %s, reason: %v"

	// ErrUnableToEncodeGuestInfo error msg
	ErrUnableToEncodeGuestInfo = "Unable to encode vmware guest info: %s, reason: %v"

	// ErrUnableToAddHardDrive error msg
	ErrUnableToAddHardDrive = "Unable to add hard drive to VM:%s, reason: %v"

	// ErrUnableToAddNetworkCard error msg
	ErrUnableToAddNetworkCard = "Unable to add network card to VM:%s, reason: %v"

	// ErrUnableToCreateDeviceChangeOp error msg
	ErrUnableToCreateDeviceChangeOp = "Unable to create device change operation for VM:%s, reason: %v"

	// ErrCloudInitFailCreation error msg
	ErrCloudInitFailCreation = "Unable to create cloud-init data for VM:%s, reason: %v"

	// ErrUnableToReconfigureVM error msg
	ErrUnableToReconfigureVM = "Unable to reconfigure VM:%s, reason: %v"

	// WarnFailedVMNotDeleted warn msg
	WarnFailedVMNotDeleted = "The failed VM:%s is not deleted because status is:%v"
)
