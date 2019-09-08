/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cpfs

import (
	"context"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/kubernetes-sigs/alibaba-cloud-csi-driver/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"path/filepath"
	"strings"
)

type nodeServer struct {
	*csicommon.DefaultNodeServer
}

type CpfsOptions struct {
	Server     string `json:"server"`
	FileSystem string `json:"fileSystem"`
	SubPath    string `json:"subPath"`
	Options    string `json:"options"`
}

const (
	CPFS_TEMP_MNTPath = "/mnt/acs_mnt/k8s_cpfs/temp" // used for create sub directory;
)

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	log.Infof("NodePublishVolume:: CPFS Mount with: %s", req.VolumeContext)

	// parse parameters
	mountPath := req.GetTargetPath()
	opt := &CpfsOptions{}
	for key, value := range req.VolumeContext {
		key = strings.ToLower(key)
		if key == "server" {
			opt.Server = value
		} else if key == "filesystem" {
			opt.FileSystem = value
		} else if key == "subpath" {
			opt.SubPath = value
		} else if key == "options" {
			opt.Options = value
		}
	}
	if mountPath == "" {
		return nil, errors.New("mountPath is empty")
	}
	if opt.Server == "" {
		return nil, errors.New("server is empty")
	}
	if opt.FileSystem == "" {
		return nil, errors.New("FileSystem is empty")
	}
	if opt.SubPath == "" {
		opt.SubPath = "/"
	}
	if !strings.HasPrefix(opt.SubPath, "/") {
		opt.SubPath = filepath.Join("/", opt.SubPath)
	}

	if utils.IsMounted(mountPath) {
		log.Infof("CPFS, Mount Path Already Mount, path: %s", mountPath)
		return &csi.NodePublishVolumeResponse{}, nil
	}

	// Create Mount Path
	if err := utils.CreateDest(mountPath); err != nil {
		return nil, errors.New("Cpfs, Mount error with create Path fail: " + mountPath)
	}

	// Do mount
	mntCmd := fmt.Sprintf("mount -t lustre %s:/%s%s %s", opt.Server, opt.FileSystem, opt.SubPath, mountPath)
	if opt.Options != "" {
		mntCmd = fmt.Sprintf("mount -t lustre -o %s %s:/%s%s %s", opt.Options, opt.Server, opt.FileSystem, opt.SubPath, mountPath)
	}
	_, err := utils.Run(mntCmd)
	if err != nil && opt.SubPath != "/" && strings.Contains(err.Error(), "No such file or directory") {
		createCpfsSubDir(opt.Options, opt.Server, opt.FileSystem, opt.SubPath, req.VolumeId)
		if _, err := utils.Run(mntCmd); err != nil {
			log.Errorf("Cpfs, Mount Cpfs after create subDirectory fail: %s", err.Error())
			return nil, errors.New("Cpfs, Mount Cpfs after create subDirectory fail: %s" + err.Error())
		}
	} else if err != nil {
		log.Errorf("Cpfs, Mount Cpfs fail: %s", err.Error())
		return nil, errors.New("Cpfs, Mount Cpfs fail: %s" + err.Error())
	}

	// check mount
	if !utils.IsMounted(mountPath) {
		return nil, errors.New("Check mount fail after mount: " + mountPath)
	}
	log.Infof("NodePublishVolume:: Mount success on mountpoint: %s, with Command: %s", mountPath, mntCmd)

	doCpfsConfig()
	return &csi.NodePublishVolumeResponse{}, nil
}

func doCpfsConfig() {
	configCmd := fmt.Sprintf("lctl set_param osc.*.max_rpcs_in_flight=256;lctl set_param osc.*.max_pages_per_rpc=1024;lctl set_param lov.*.target_obds.*osc*.max_rpcs_in_flight=256;lctl set_param mdc.*.max_rpcs_in_flight=256;lctl set_param lmv.*.target_obds.*.max_rpcs_in_flight=256")
	if _, err := utils.Run(configCmd); err != nil {
		log.Errorf("Cpfs, doCpfsConfig fail with error: %s", err.Error())
	}
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	log.Infof("NodeUnpublishVolume:: Starting Umount Cpfs: %s", req.TargetPath)
	mountPoint := req.TargetPath
	if !utils.IsMounted(mountPoint) {
		log.Infof("Path not mounted, skipped: %s", mountPoint)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	umntCmd := fmt.Sprintf("umount %s", mountPoint)
	if _, err := utils.Run(umntCmd); err != nil {
		return nil, errors.New("Cpfs, Umount cpfs Fail: " + err.Error())
	}

	log.Infof("Umount cpfs Successful on: %s", mountPoint)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(
	ctx context.Context,
	req *csi.NodeStageVolumeRequest) (
	*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ns *nodeServer) NodeUnstageVolume(
	ctx context.Context,
	req *csi.NodeUnstageVolumeRequest) (
	*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (
	*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
