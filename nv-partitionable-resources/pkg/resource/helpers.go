package resource

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	nvdevicelib "github.com/kubernetes-sigs/wg-device-management/nv-partitionable-resources/pkg/nvdevice"
)

// PerGpuAllocatableDevices is an alias of nvdevicelib.PerGpuAllocatableDevices
type PerGpuAllocatableDevices nvdevicelib.PerGpuAllocatableDevices

// AllocatableDevices is an alias of nvdevicelib.AllocatableDevices
type AllocatableDevices nvdevicelib.AllocatableDevices

// GpuInfo is an alias of nvdevicelib.GpuInfo
type GpuInfo nvdevicelib.GpuInfo

// MigInfo is an alias of nvdevicelib.MigInfo
type MigInfo nvdevicelib.MigInfo

// ToResourceSliceSpec converts a list of PerGpuAllocatableDevices to a ResourceSliceSpec
func (pgads PerGpuAllocatableDevices) ToResourceSliceSpec() *ResourceSliceSpec {
	deviceMixinsMap := make(map[string]DeviceMixin)
	deviceCounterConsumptionMixinsMap := make(map[string]DeviceCounterConsumptionMixin)
	sharedCountersMixinsMap := make(map[string]CounterSetMixin)
	devicesMap := make(map[string]Device)
	indexToProductNameMap := make(map[int]string)

	// Generate a device mixin for all common system attributes.
	systemAttributesName := toRFC1123Compliant("system-attributes")
	deviceMixinsMap[systemAttributesName] = DeviceMixin{
		Name: "system-attributes",
		Attributes: map[QualifiedName]DeviceAttribute{
			"driverVersion": {
				VersionValue: ptr.To(nvdevicelib.PerGpuAllocatableDevices(pgads).SystemInfo.DriverVersion),
			},
			"cudaDriverVersion": {
				VersionValue: ptr.To(nvdevicelib.PerGpuAllocatableDevices(pgads).SystemInfo.CudaDriverVersion),
			},
		},
	}

	// Track the biggest memory slice discovered per GPU so we can include that
	// in the device specs we eventually generate.
	maxMemorySlice := make(map[int]uint32)

	// Track the maximum capacity of all MIG resources (on a per gpu basis) so
	// that we can apply them to each full GPU later on.
	maxCapacities := make(map[int]map[QualifiedName]DeviceCapacity)

	maxMemory := make(map[int]DeviceCapacity)

	// Loop through all discovered devices and generate mixins and devices from them.
	for i := range pgads.Devices {
		for _, d := range pgads.Devices[i] {
			// If this is a full GPU ...
			if d.Gpu != nil {
				indexToProductNameMap[d.Gpu.Index] = d.Gpu.ProductName
				// Generate common attributes for all instances of the current GPU type.
				commonGpuAttributesName := toRFC1123Compliant(fmt.Sprintf("common-gpu-%s-attributes", d.Gpu.ProductName))
				deviceMixinsMap[commonGpuAttributesName] = DeviceMixin{
					Name: commonGpuAttributesName,
					Attributes: map[QualifiedName]DeviceAttribute{
						"type": {
							StringValue: ptr.To("gpu"),
						},
						"architecture": {
							StringValue: ptr.To(d.Gpu.Architecture),
						},
						"brand": {
							StringValue: ptr.To(d.Gpu.Brand),
						},
						"productName": {
							StringValue: ptr.To(d.Gpu.ProductName),
						},
						"cudaComputeCapability": {
							StringValue: ptr.To(d.Gpu.CudaComputeCapability),
						},
					},
				}

				// Generate common capacities for all instance of the current GPU type.
				// NOTE: These will be patched up after computing the max GMI capacities required.
				commonGpuCapacitiesName := toRFC1123Compliant(fmt.Sprintf("common-gpu-%s-capacities", d.Gpu.ProductName))
				deviceMixinsMap[commonGpuCapacitiesName] = DeviceMixin{
					Name: commonGpuCapacitiesName,
					Capacity: map[QualifiedName]DeviceCapacity{
						"memory": {
							Quantity: *resource.NewQuantity(int64(d.Gpu.MemoryBytes), resource.BinarySI),
						},
					},
				}

				// Generate attributes specific to this particular instance of a GPU.
				specificGpuAttributesName := toRFC1123Compliant(fmt.Sprintf("specific-gpu-%d-attributes", d.Gpu.Index))
				deviceMixinsMap[specificGpuAttributesName] = DeviceMixin{
					Name: specificGpuAttributesName,
					Attributes: map[QualifiedName]DeviceAttribute{
						"index": {
							IntValue: ptr.To(int64(d.Gpu.Index)),
						},
						"minor": {
							IntValue: ptr.To(int64(d.Gpu.Minor)),
						},
						"uuid": {
							StringValue: ptr.To(d.Gpu.UUID),
						},
					},
				}

				sharedCountersMixinName := toRFC1123Compliant(fmt.Sprintf("gpu-%s-counter-set", d.Gpu.ProductName))
				sharedCountersMixinsMap[sharedCountersMixinName] = CounterSetMixin{
					Name:     sharedCountersMixinName,
					Counters: make(map[string]Counter),
				}
			}

			// If this is a MIG device ...
			if d.Mig != nil {
				// Generate common attributes for all MIG instances of the current GPU type.
				commonMigAttributesName := toRFC1123Compliant(fmt.Sprintf("common-mig-%s-attributes", d.Mig.Parent.ProductName))
				deviceMixinsMap[commonMigAttributesName] = DeviceMixin{
					Name: commonMigAttributesName,
					Attributes: map[QualifiedName]DeviceAttribute{
						"type": {
							StringValue: ptr.To("mig"),
						},
						"architecture": {
							StringValue: ptr.To(d.Mig.Parent.Architecture),
						},
						"brand": {
							StringValue: ptr.To(d.Mig.Parent.Brand),
						},
						"productName": {
							StringValue: ptr.To(d.Mig.Parent.ProductName),
						},
						"cudaComputeCapability": {
							StringValue: ptr.To(d.Mig.Parent.CudaComputeCapability),
						},
					},
				}

				// Generate MIG attributes specific to this particular instance of a GPU.
				specificGpuMigAttributesName := toRFC1123Compliant(fmt.Sprintf("specific-gpu-%d-mig-attributes", d.Mig.Parent.Index))
				deviceMixinsMap[specificGpuMigAttributesName] = DeviceMixin{
					Name: specificGpuMigAttributesName,
					Attributes: map[QualifiedName]DeviceAttribute{
						"parentIndex": {
							IntValue: ptr.To(int64(d.Mig.Parent.Index)),
						},
						"parentMinor": {
							IntValue: ptr.To(int64(d.Mig.Parent.Minor)),
						},
						"parentUUID": {
							StringValue: ptr.To(d.Mig.Parent.UUID),
						},
					},
				}

				// Generate a mixin for current the MIG profile specific to this particular type of GPU.
				info := d.Mig.GIProfileInfo
				commonMigMixinName := toRFC1123Compliant(fmt.Sprintf("common-mig-%s-%s", d.Mig.Profile, d.Mig.Parent.ProductName))
				memory := DeviceCapacity{
					Quantity: *resource.NewQuantity(int64(info.MemorySizeMB*1024*1024), resource.BinarySI),
				}
				deviceMixinsMap[commonMigMixinName] = DeviceMixin{
					Name: commonMigMixinName,
					Attributes: map[QualifiedName]DeviceAttribute{
						"profile": {
							StringValue: ptr.To(d.Mig.Profile.String()),
						},
					},
					Capacity: map[QualifiedName]DeviceCapacity{
						"multiprocessors": {
							Quantity: *resource.NewQuantity(int64(info.MultiprocessorCount), resource.BinarySI),
						},
						"copy-engines": {
							Quantity: *resource.NewQuantity(int64(info.CopyEngineCount), resource.BinarySI),
						},
						"decoders": {
							Quantity: *resource.NewQuantity(int64(info.DecoderCount), resource.BinarySI),
						},
						"encoders": {
							Quantity: *resource.NewQuantity(int64(info.EncoderCount), resource.BinarySI),
						},
						"jpeg-engines": {
							Quantity: *resource.NewQuantity(int64(info.JpegCount), resource.BinarySI),
						},
						"ofa-engines": {
							Quantity: *resource.NewQuantity(int64(info.OfaCount), resource.BinarySI),
						},
						"memory": memory,
					},
				}
				commonMigDeviceCounterConsumptionName := toRFC1123Compliant(fmt.Sprintf("common-mig-%s-%s", d.Mig.Profile, d.Mig.Parent.ProductName))
				// In this example the advertised capacity is always equal to the consumed counters. So we just create the
				// latter from the former.
				deviceCounterConsumptionMixinsMap[commonMigDeviceCounterConsumptionName] = DeviceCounterConsumptionMixin{
					Name:     commonMigDeviceCounterConsumptionName,
					Counters: capacityToCounters(deviceMixinsMap[commonMigMixinName].Capacity),
				}

				m, found := maxMemory[d.Mig.Parent.Index]
				if !found || memory.Quantity.Cmp(m.Quantity) > 0 {
					maxMemory[d.Mig.Parent.Index] = memory
				}

				// Track the maxCapacities of all capacity consumed by any MIG
				// device on the current full GPU so that we can apply these
				// max capacities to the full GPU later on.
				if maxCapacities[d.Mig.Parent.Index] == nil {
					maxCapacities[d.Mig.Parent.Index] = make(map[QualifiedName]DeviceCapacity)
				}
				for k, v := range deviceMixinsMap[commonMigMixinName].Capacity {
					if k == "memory" {
						continue
					}
					if ptr.To(maxCapacities[d.Mig.Parent.Index][k].Quantity).Cmp(v.Quantity) <= 0 {
						maxCapacities[d.Mig.Parent.Index][k] = DeviceCapacity{
							Quantity: v.Quantity,
						}
					}
				}

				// Generate a mixin to represent all of the memory slices consumed by the current MIG device.
				placement := d.Mig.MemorySlices
				memorySlicesSuffix := fmt.Sprintf("%d", placement.Start)
				if placement.Size > 1 {
					memorySlicesSuffix = fmt.Sprintf("%s-%d", memorySlicesSuffix, placement.Start+placement.Size-1)
				}
				memorySlicesName := toRFC1123Compliant(fmt.Sprintf("memory-slices-%s", memorySlicesSuffix))
				deviceMixinsMap[memorySlicesName] = DeviceMixin{
					Name:     memorySlicesName,
					Capacity: map[QualifiedName]DeviceCapacity{},
				}
				for i := placement.Start; i < placement.Start+placement.Size; i++ {
					sliceName := QualifiedName(fmt.Sprintf("memorySlice%d", i))
					deviceMixinsMap[memorySlicesName].Capacity[sliceName] = DeviceCapacity{
						Quantity: *resource.NewQuantity(1, resource.BinarySI),
					}
				}
				deviceCounterConsumptionMixinsMap[memorySlicesName] = DeviceCounterConsumptionMixin{
					Name:     memorySlicesName,
					Counters: capacityToCounters(deviceMixinsMap[memorySlicesName].Capacity),
				}

				// Track the max memory slice consumed by any MIG device on the
				// current full GPU so that we can apply it to the full GPU
				// later on.
				maxMemorySlice[d.Mig.Parent.Index] = max(maxMemorySlice[d.Mig.Parent.Index], placement.Start+placement.Size-1)

				// Generate the actual MIG device spec (not a mixin).
				migDeviceName := toRFC1123Compliant(fmt.Sprintf("gpu-%d-mig-%s-%s", d.Mig.Parent.Index, d.Mig.Profile, memorySlicesSuffix))
				devicesMap[migDeviceName] = Device{
					Name: migDeviceName,
					Includes: []DeviceMixinRef{
						{
							Name: systemAttributesName,
						},
						{
							Name: commonMigAttributesName,
						},
						{
							Name: commonMigMixinName,
						},
						{
							Name: specificGpuMigAttributesName,
						},
						{
							Name: memorySlicesName,
						},
					},
					ConsumesCounters: []DeviceCounterConsumption{
						{
							CounterSet: toRFC1123Compliant(fmt.Sprintf("gpu-%d-counter-set", d.Mig.Parent.Index)),
							Includes: []DeviceCounterConsumptionMixinRef{
								{
									Name: commonMigDeviceCounterConsumptionName,
								},
								{
									Name: memorySlicesName,
								},
							},
						},
					},
				}
			}
		}
	}

	// Loop through all full GPus again to add device specs for them.
	// We need to do this in a second loop to make use of the maxMemorySlice and
	// maxCapacities objects constructed in the first loop.
	for i := range pgads.Devices {
		for _, d := range pgads.Devices[i] {
			if d.Gpu != nil {
				// Recreate the names of the mixins we need to include in each concrete device.
				commonGpuAttributesName := toRFC1123Compliant(fmt.Sprintf("common-gpu-%s-attributes", d.Gpu.ProductName))
				commonGpuCapacitiesName := toRFC1123Compliant(fmt.Sprintf("common-gpu-%s-capacities", d.Gpu.ProductName))
				specificGpuAttributesName := toRFC1123Compliant(fmt.Sprintf("specific-gpu-%d-attributes", d.Gpu.Index))
				memorySlicesName := toRFC1123Compliant(fmt.Sprintf("memory-slices-%d-%d", 0, maxMemorySlice[d.Gpu.Index]))
				commonGpuDeviceCapacityConsumptionName := toRFC1123Compliant(fmt.Sprintf("common-gpu-%s-capacity-consumption", d.Gpu.ProductName))
				counterSetName := toRFC1123Compliant(fmt.Sprintf("gpu-%d-counter-set", d.Gpu.Index))

				// Patch the specificGpuTypeName mixin with its capacities.
				for k, v := range maxCapacities[d.Gpu.Index] {
					deviceMixinsMap[commonGpuCapacitiesName].Capacity[k] = v
				}

				deviceCounterConsumptionMixinsMap[commonGpuDeviceCapacityConsumptionName] = DeviceCounterConsumptionMixin{
					Name:     commonGpuDeviceCapacityConsumptionName,
					Counters: capacityToCounters(deviceMixinsMap[commonGpuCapacitiesName].Capacity),
				}

				// Add each full GPU as a device in terms of its mixins.
				specificGpuName := toRFC1123Compliant(fmt.Sprintf("gpu-%d", d.Gpu.Index))
				devicesMap[specificGpuName] = Device{
					Name: specificGpuName,
					Includes: []DeviceMixinRef{
						{
							Name: systemAttributesName,
						},
						{
							Name: commonGpuAttributesName,
						},
						{
							Name: commonGpuCapacitiesName,
						},
						{
							Name: specificGpuAttributesName,
						},
						{
							Name: memorySlicesName,
						},
					},
					ConsumesCounters: []DeviceCounterConsumption{
						{
							CounterSet: counterSetName,
							Includes: []DeviceCounterConsumptionMixinRef{
								{
									Name: commonGpuDeviceCapacityConsumptionName,
								},
								{
									Name: memorySlicesName,
								},
							},
						},
					},
				}
			}
		}
	}

	var sharedCounters []CounterSet
	for index, caps := range maxCapacities {
		productName := indexToProductNameMap[index]
		sharedCountersMixinName := toRFC1123Compliant(fmt.Sprintf("gpu-%s-counter-set", productName))
		sharedCountersMixin := sharedCountersMixinsMap[sharedCountersMixinName]
		sharedCountersMixin.Counters = capacityToCounters(caps)

		maxIndexMemSlice := maxMemorySlice[index]
		for j := uint32(0); j <= maxIndexMemSlice; j++ {
			sliceName := fmt.Sprintf("memorySlice%d", j)
			sharedCountersMixin.Counters[sliceName] = Counter{
				Value: *resource.NewQuantity(1, resource.BinarySI),
			}
			sharedCountersMixin.Counters["memory"] = capacityToCounter(maxMemory[index])
		}
		sharedCountersMixinsMap[sharedCountersMixinName] = sharedCountersMixin

		sharedCounters = append(sharedCounters, CounterSet{
			Name: toRFC1123Compliant(fmt.Sprintf("gpu-%d-counter-set", index)),
			Includes: []SharedCountersMixinRef{
				{
					Name: sharedCountersMixinName,
				},
			},
		})
	}

	var deviceMixins []DeviceMixin
	for _, v := range deviceMixinsMap {
		deviceMixins = append(deviceMixins, v)
	}

	var deviceCounterConsumptionMixins []DeviceCounterConsumptionMixin
	for _, v := range deviceCounterConsumptionMixinsMap {
		deviceCounterConsumptionMixins = append(deviceCounterConsumptionMixins, v)
	}

	var sharedCountersMixins []CounterSetMixin
	for _, v := range sharedCountersMixinsMap {
		sharedCountersMixins = append(sharedCountersMixins, v)
	}

	var devices []Device
	for _, v := range devicesMap {
		devices = append(devices, v)
	}

	spec := ResourceSliceSpec{
		SharedCounters: sharedCounters,
		Mixins: &ResourceSliceMixins{
			Device:           deviceMixins,
			ConsumesCounters: deviceCounterConsumptionMixins,
			SharedCounters:   sharedCountersMixins,
		},
		Devices: devices,
	}

	return &spec
}

// Flatten unrolls a spec into a list of flat devices resolving any mixins into
// inline attributes and capacities.
func (s *ResourceSliceSpec) Flatten() (*ResourceSliceSpec, error) {
	// Declare a new resource slice spec to hold the flattened result.
	flattenedSpec := ResourceSliceSpec{}

	// Build a map of all the mixins by name. Return an error if any names are duplicated.
	mixinMap := make(map[string]*DeviceMixin)
	for _, m := range s.Mixins.Device {
		if _, exists := mixinMap[m.Name]; exists {
			return nil, fmt.Errorf("duplicate mixin name detected: %s", m.Name)
		}
		mixinMap[m.Name] = &m
	}

	deviceCounterConsumptionMixinMap := make(map[string]*DeviceCounterConsumptionMixin)
	for _, m := range s.Mixins.ConsumesCounters {
		if _, exists := deviceCounterConsumptionMixinMap[m.Name]; exists {
			return nil, fmt.Errorf("duplicate device counter consumption mixin name detected: %s", m.Name)
		}
		deviceCounterConsumptionMixinMap[m.Name] = &m
	}

	sharedCountersMixinMap := make(map[string]*CounterSetMixin)
	for _, m := range s.Mixins.SharedCounters {
		if _, exists := sharedCountersMixinMap[m.Name]; exists {
			return nil, fmt.Errorf("duplicate shared counters mixin name detected: %s", m.Name)
		}
		sharedCountersMixinMap[m.Name] = &m
	}

	// Flatten each device and add it back to the flattened spec.
	for _, d := range s.Devices {
		flattened := &DeviceMixin{
			Attributes: make(map[QualifiedName]DeviceAttribute),
			Capacity:   make(map[QualifiedName]DeviceCapacity),
		}
		for _, m := range d.Includes {
			for k, v := range mixinMap[m.Name].Attributes {
				flattened.Attributes[k] = v
			}
			for k, v := range mixinMap[m.Name].Capacity {
				flattened.Capacity[k] = v
			}
		}
		for k, v := range d.Attributes {
			flattened.Attributes[k] = v
		}
		for k, v := range d.Capacity {
			flattened.Capacity[k] = v
		}

		d.Includes = nil
		d.Attributes = flattened.Attributes
		d.Capacity = flattened.Capacity

		flattenedDeviceCounterConsumptions := make([]DeviceCounterConsumption, 0)
		for _, cc := range d.ConsumesCounters {
			flattenedDCC := DeviceCounterConsumption{
				CounterSet: cc.CounterSet,
				Counters:   make(map[string]Counter),
			}
			for _, m := range cc.Includes {
				for k, v := range deviceCounterConsumptionMixinMap[m.Name].Counters {
					flattenedDCC.Counters[k] = v
				}
			}
			for k, v := range cc.Counters {
				flattenedDCC.Counters[k] = v
			}
			flattenedDeviceCounterConsumptions = append(flattenedDeviceCounterConsumptions, flattenedDCC)
		}
		d.ConsumesCounters = flattenedDeviceCounterConsumptions

		flattenedSpec.Devices = append(flattenedSpec.Devices, d)
	}

	for _, sc := range s.SharedCounters {
		if sc.Counters == nil {
			sc.Counters = make(map[string]Counter)
		}
		for _, entry := range sc.Includes {
			for k, v := range sharedCountersMixinMap[entry.Name].Counters {
				sc.Counters[k] = v
			}
		}
		sc.Includes = nil
		flattenedSpec.SharedCounters = append(flattenedSpec.SharedCounters, sc)
	}

	return &flattenedSpec, nil
}

// toRFC1123Compliant converts the incoming string to a valid RFC1123 DNS domain name.
func toRFC1123Compliant(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with '-'
	re := regexp.MustCompile(`[^a-z0-9-.]`)
	name = re.ReplaceAllString(name, "-")

	// Trim leading/trailing '-'
	name = strings.Trim(name, "-")

	// Trim trailing '.'
	name = strings.TrimSuffix(name, ".")

	// Truncate to 253 characters
	if len(name) > 253 {
		name = name[:253]
	}

	return name
}

func capacityToCounters(capacity map[QualifiedName]DeviceCapacity) map[string]Counter {
	counters := make(map[string]Counter)
	for k, v := range capacity {
		counters[string(k)] = capacityToCounter(v)
	}
	return counters
}

func capacityToCounter(cap DeviceCapacity) Counter {
	return Counter{
		Value: cap.Quantity,
	}
}
