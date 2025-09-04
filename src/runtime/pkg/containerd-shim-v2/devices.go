// Copyright (c) 2025 Nvidia
//
// SPDX-License-Identifier: Apache-2.0
//

package containerdshim

import (
	"encoding/json"
	"fmt"

	"github.com/kata-containers/kata-containers/src/runtime/pkg/oci"
)

const (
	resourceAnnotation  = "io.katacontainers.pod-resources"
	nameAnnotation      = "io.kubernetes.cri.sandbox-name"
	namespaceAnnotation = "io.kubernetes.cri.sandbox-namespace"
)

type PodResourceSpec struct {
	Containers     map[string]ResourceSpec `json:"containers"`
	InitContainers map[string]ResourceSpec `json:"initContainers"`
}

// ResourceSpec is a string maps of the form resourceName : quantity,
// originating from the Pod spec
type ResourceSpec map[string]string

// ResolveRequest defines the request send to the resolver.
type ResolveRequest struct {
	// SpecName is the name of the device as called out in the pod spec
	// e.g. vendor.com/gpu
	SpecName string `json:"spec_name"`
	// UUID identifies the Pod
	UUID string `json:"uuid"`
	// VirtualDeviceIDs are used for maintaining a mapping
	// between higher level device IDs and physical devices.
	// Empty strings can be used if the ID is unknown
	// Length of this list is the number of devices that need
	// to be resolved
	VirtualDeviceIDs []string `json:"virtual_device_ids"`
}

func getPodResourceSpec(anno map[string]string) (*PodResourceSpec, error) {
	specJson, found := anno[resourceAnnotation]
	if !found {
		return nil, nil
	}

	var prs PodResourceSpec
	if err := json.Unmarshal([]byte(specJson), &prs); err != nil {
		return nil, err
	}

	return &prs, nil
}

func resolveCDIAnnotations(anno map[string]string, config *oci.RuntimeConfig) error {
	prs, err := getPodResourceSpec(anno)
	if err != nil {
		return err
	}

	if prs == nil {
		// no annotations to process
		return nil
	}

	// convert resolver list into a map
	resolvers := make(map[string]*oci.Resolver)
	for _, res := range config.ProxyCDIResolvers {
		resolvers[res.SpecName] = &res
	}

	if len(resolvers) == 0 {
		return fmt.Errorf("%s present, but no resolvers", resourceAnnotation)
	}

	for _, crs := range prs.Containers {
		for device, count := range crs {
			res := resolvers[device]
			// add CDI annotations
			proxyResolveCDI(res, count, anno)
		}
	}

	return nil
}

func proxyResolveCDI(res *oci.Resolver, count string, anno map[string]string) error {
	numDevices, err := strconv.Atoi(count)
	if err != nil {
		return err
	}
	podName := anno[nameAnnotation]
	podNs := anno[namespaceAnnotation]
	var virtDevs []string
	for ix := 0; ix < numDevices; ix++ {
		virtDevs = append(virtDevs, "")
	}
	req := &ResolveRequest{
		SpecName:         res.SpecName,
		UUID:             podNs + "_" + podName,
		VirtualDeviceIDs: virtDevs,
	}
	return nil
}
