// Code generated by "stringer -type=blencDirNodeState -output blencdirnodestate.generated.go"; DO NOT EDIT

package graph

import "fmt"

const _blencDirNodeState_name = "blencDirNodeStateUnloadedblencDirNodeStateLoadingblencDirNodeStateIdleblencDirNodeStateSaveRequestedblencDirNodeStateSavingblencDirNodeStateLoadError"

var _blencDirNodeState_index = [...]uint8{0, 25, 49, 70, 100, 123, 149}

func (i blencDirNodeState) String() string {
	if i < 0 || i >= blencDirNodeState(len(_blencDirNodeState_index)-1) {
		return fmt.Sprintf("blencDirNodeState(%d)", i)
	}
	return _blencDirNodeState_name[_blencDirNodeState_index[i]:_blencDirNodeState_index[i+1]]
}