package resource

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// QualifiedName is the name of a device attribute or capacity.
//
// Attributes and capacities are defined either by the owner of the specific
// driver (usually the vendor) or by some 3rd party (e.g. the Kubernetes
// project). Because they are sometimes compared across devices, a given name
// is expected to mean the same thing and have the same type on all devices.
//
// Names must be either a C identifier (e.g. "theName") or a DNS subdomain
// followed by a slash ("/") followed by a C identifier
// (e.g. "dra.example.com/theName"). Names which do not include the
// domain prefix are assumed to be part of the driver's domain. Attributes
// or capacities defined by 3rd parties must include the domain prefix.
//
// The maximum length for the DNS subdomain is 63 characters (same as
// for driver names) and the maximum length of the C identifier
// is 32.
type QualifiedName string

// ResourceSliceSpec contains the information published by the driver in one ResourceSlice.
// +k8s:deepcopy-gen=true
type ResourceSliceSpec struct {
	// Devices lists some or all of the devices in this pool.
	//
	// Must not have more than 128 entries.
	//
	// +optional
	// +listType=atomic
	Devices []Device `json:"devices,omitempty"`

	// CapacityPools defines a list of capacity pools, each of which
	// has a name and a list of capacities available in the pool.
	//
	// The names of the pools must be unique in the ResourceSlice.
	//
	// The maximum number of pools is 32.
	//
	// +optional
	// listType=atomic
	SharedCounters []CounterSet `json:"sharedCounters,omitempty"`

	// Mixins defines the mixins available for devices and capacity pools
	// in the ResourceSlice.
	//
	// +optional
	Mixins *ResourceSliceMixins `json:"mixins,omitempty"`
}

// ResourceSliceMixins defines mixins for the ResourceSlice.
// +k8s:deepcopy-gen=true
type ResourceSliceMixins struct {
	// Device represents a list of device mixins, i.e. a collection of
	// shared attributes and capacities that an actual device can "include"
	// to extend the set of attributes and capacities it already defines.
	//
	// The main purposes of these mixins is to reduce the memory footprint
	// of devices since they can reference the mixins provided here rather
	// than duplicate them.
	//
	// The total number of device mixins, device capacity consumption mixins,
	// capacity pool mixins, basic devices, and composite devices must be
	// less than 128.
	//
	// +optional
	// +listType=atomic
	Device []DeviceMixin `json:"device,omitempty"`

	// DeviceCapacityConsumption represents a list of capacity
	// consumption mixins, each of which contains a set of capacities
	// that a device will consume from a capacity pool.
	//
	// This makes it possible to define a set of shared capacities that
	// are not tied to a specific pool. The pool is inferred by context
	// in which the DeviceCapacityConsumptionMixin is referenced from
	// the device.
	//
	// The total number of device mixins, device capacity consumption mixins,
	// capacity pool mixins, basic devices, and composite devices must be
	// less than 128.
	//
	// +optional
	// +listType=atomic
	ConsumesCounters []DeviceCounterConsumptionMixin `json:"consumesCounters,omitempty"`

	// CapacityPool represents a list of capacity pool mixins, i.e.
	// a collection of capacities that a CapacityPool can "include"
	// to extend the set of capacities it already defines.
	//
	// The main purposes of these mixins is to reduce the memory footprint
	// of capacity pools since they can reference the mixins provided here rather
	// than duplicate them.
	//
	// The total number of device mixins, device capacity consumption mixins,
	// capacity pool mixins, basic devices, and composite devices must be
	// less than 128.
	//
	// +optional
	// +listType=atomic
	SharedCounters []CounterSetMixin `json:"sharedCounters,omitempty"`
}

// CapacityPool defines a named pool of capacities
// that are available to be used by devices defined in the
// ResourceSlice.
//
// The capacities are not allocatable by themselves, but
// can be referenced by devices. When a device is allocated,
// the capacity it uses will no longer be available for use
// by other devices.
// +k8s:deepcopy-gen=true
type CounterSet struct {
	// Name defines the name of the capacity pool.
	// It must be a DNS label.
	//
	// +required
	Name string `json:"name,omitempty"`

	// Includes defines the set of capacity pool mixins that this capacity
	// pool includes.
	//
	// The propertes of each included mixin are applied to this capacity pool in
	// order. Conflicting properties from multiple mixins are taken from the
	// last mixin listed that contains them. Properties set on the capacity pool will
	// always override properties from mixins.
	//
	// The mixins referenced here must be defined in the same
	// ResourceSlice.
	//
	// The maximum number of mixins that can be included is 8.
	//
	// +optional
	// +listType=atomic
	Includes []SharedCountersMixinRef `json:"includes,omitempty"`

	// Capacity defines the set of capacities for this capacity pool
	// The name of each capacity must be unique in that set.
	//
	// To ensure this uniqueness, capacities defined by the vendor
	// must be listed without the driver name as domain prefix in
	// their name. All others must be listed with their domain prefix.
	//
	// Capacities listed here will always take precedence over any included
	// from a mixin.
	//
	// The maximum number of capacities is 32.
	//
	// +required
	Counters map[string]Counter `json:"counters,omitempty"`
}

// DeviceMixin defines a specific device mixin for each device type.
// Besides the name, exactly one field must be set.
// +k8s:deepcopy-gen=true
type DeviceMixin struct {
	// Name is a unique identifier among all mixins managed by the driver
	// in the pool. It must be a DNS label.
	//
	// +required
	Name string `json:"name"`

	// Attributes defines the set of attributes for this mixin.
	// The name of each attribute must be unique in that set.
	//
	// To ensure this uniqueness, attributes defined by the vendor
	// must be listed without the driver name as domain prefix in
	// their name. All others must be listed with their domain prefix.
	//
	// Conflicting attributes from those provided via other mixins are
	// overwritten by the ones provided here.
	//
	// The maximum number of attributes and capacities combined is 32.
	//
	// +optional
	Attributes map[QualifiedName]DeviceAttribute `json:"attributes,omitempty"`

	// Capacity defines the set of capacities for this mixin.
	// The name of each capacity must be unique in that set.
	//
	// To ensure this uniqueness, capacities defined by the vendor
	// must be listed without the driver name as domain prefix in
	// their name. All others must be listed with their domain prefix.
	//
	// Conflicting capacities from those provided via other mixins are
	// overwritten by the ones provided here.
	//
	// The maximum number of attributes and capacities combined is 32.
	//
	// +optional
	Capacity map[QualifiedName]DeviceCapacity `json:"capacity,omitempty"`
}

// DeviceCapacityConsumptionMixin defines a mixin that composite
// devices can include to adopt the consuption capacity defined in
// the mixin.
// +k8s:deepcopy-gen=true
type DeviceCounterConsumptionMixin struct {
	// Name is a unique identifier among all device capacity consumption
	// mixins in the ResourceSlice. It must be a DNS label.
	//
	// +required
	Name string `json:"name,omitempty"`

	// Capacity defines a set of capacities
	// that a device will consume from a capacity pool.
	//
	// The capacity pool is not specified here but is determined
	// from the context in which the DeviceCapacityConsumptionMixin
	// is referenced from the device.
	//
	// The maximum number of capacities is 32
	//
	// +required
	Counters map[string]Counter `json:"counters,omitempty"`
}

// Device represents one individual hardware instance that can be selected based
// on its attributes. Besides the name, exactly one field must be set.
// +k8s:deepcopy-gen=true
type Device struct {
	// Name is unique identifier among all devices managed by
	// the driver in the pool. It must be a DNS label.
	//
	// +required
	Name string `json:"name"`

	// Includes defines the set of device mixins that this device includes.
	//
	// The propertes of each included mixin are applied to this device in
	// order. Conflicting properties from multiple mixins are taken from the
	// last mixin listed that contains them.
	//
	// The maximum number of mixins that can be included is 8.
	//
	// +optional
	Includes []DeviceMixinRef `json:"includes,omitempty"`

	// Attributes defines the set of attributes for this device.
	// The name of each attribute must be unique in that set.
	//
	// To ensure this uniqueness, attributes defined by the vendor
	// must be listed without the driver name as domain prefix in
	// their name. All others must be listed with their domain prefix.
	//
	// Conflicting attributes from those provided via mixins are
	// overwritten by the ones provided here.
	//
	// The maximum number of attributes and capacities combined is 32.
	//
	// +optional
	Attributes map[QualifiedName]DeviceAttribute `json:"attributes,omitempty"`

	// Capacity defines the set of capacities for this device.
	// The name of each capacity must be unique in that set.
	//
	// To ensure this uniqueness, capacities defined by the vendor
	// must be listed without the driver name as domain prefix in
	// their name. All others must be listed with their domain prefix.
	//
	// Conflicting capacities from those provided via mixins are
	// overwritten by the ones provided here.
	//
	// The maximum number of attributes and capacities combined is 32.
	//
	// +optional
	Capacity map[QualifiedName]DeviceCapacity `json:"capacity,omitempty"`

	// ConsumesCapacity defines a list of references to capacity
	// pools and the set of capacities that the device will
	// consume from those pools.
	//
	// The capacities can be defined either by referencing one
	// or more DeviceCapacityConsumptionMixins by listing
	// the capacities directly. The latter will always override
	// any capacities coming in from the mixins.
	//
	// The maximum number of device capacity consumption entries
	// is 32. This is the same as the maximum number of capacity
	// pools allowed in a ResourceSlice.
	//
	// +required
	// +listType=atomic
	ConsumesCounters []DeviceCounterConsumption `json:"consumesCounters,omitempty"`
}

// CapacityPoolMixin defines a mixin that a capacity pool can include.
// +k8s:deepcopy-gen=true
type CounterSetMixin struct {
	// Name is a unique identifier among all capacity pool mixins in the ResourceSlice.
	// It must be a DNS label.
	//
	// +required
	Name string `json:"name,omitempty"`

	// Capacity defines the set of capacities for this mixin.
	// The name of each capacity must be unique in that set.
	//
	// To ensure this uniqueness, capacities defined by the vendor
	// must be listed without the driver name as domain prefix in
	// their name. All others must be listed with their domain prefix.
	//
	// Conflicting capacities from those provided via other mixins are
	// overwritten by the ones provided here.
	//
	// The maximum number of capacities is 32.
	//
	// +required
	Counters map[string]Counter `json:"counters,omitempty"`
}

// DeviceMixinRef defines a reference to a device mixin.
// +k8s:deepcopy-gen=true
type DeviceMixinRef struct {
	// Name refers to the name of a device mixin in the pool.
	//
	// +required
	Name string `json:"name"`
}

// DeviceRef defines a reference to a device.
// +k8s:deepcopy-gen=true
type DeviceRef struct {
	// Name refers to the name of a device in the pool.
	//
	// +required
	Name string `json:"name"`
}

// SharedCountersMixinRef
// +k8s:deepcopy-gen=true
type SharedCountersMixinRef struct {
	// Name refers to the name of a device in the pool.
	//
	// +required
	Name string `json:"name"`
}

// +k8s:deepcopy-gen=true
type DeviceCounterConsumptionMixinRef struct {
	// Name is the name of a DeviceCapacityConsumptionMixin.
	//
	// +required
	Name string `json:"name"`
}

// DeviceCapacityConsumption defines a set of capacities that
// a device will consume from a capacity pool.
// +k8s:deepcopy-gen=true
type DeviceCounterConsumption struct {
	// CapacityPool defines the capacity pool from which the
	// capacities defined (either directly or through a mixin)
	// will be consumed from.
	//
	// +required
	CounterSet string `json:"counterSet,omitempty"`

	// Includes defines a list of references to DeviceCapacityConsumptionMixins.
	// The capacities listed in these will be included in among the
	// capacities that will be consumed by the device.
	//
	// Capacities listed directly will override any capacities coming
	// from mixins.
	//
	// The maximum number of mixins that can be included is 8.
	//
	// +optional
	Includes []DeviceCounterConsumptionMixinRef `json:"includes,omitempty"`

	// Capacity defines the capacity that will be consumed by
	// the device.
	//
	// Capacities listed here will override any capacities that
	// are also defined in any of the referenced mixins.
	//
	// The maximum number of capacities is 32.
	//
	// +optional
	Counters map[string]Counter `json:"counters,omitempty"`
}

// DeviceAttribute must have exactly one field set.
// +k8s:deepcopy-gen=true
type DeviceAttribute struct {
	// The Go field names below have a Value suffix to avoid a conflict between the
	// field "String" and the corresponding method. That method is required.
	// The Kubernetes API is defined without that suffix to keep it more natural.

	// IntValue is a number.
	//
	// +optional
	// +oneOf=ValueType
	IntValue *int64 `json:"int,omitempty"`

	// BoolValue is a true/false value.
	//
	// +optional
	// +oneOf=ValueType
	BoolValue *bool `json:"bool,omitempty"`

	// StringValue is a string. Must not be longer than 64 characters.
	//
	// +optional
	// +oneOf=ValueType
	StringValue *string `json:"string,omitempty"`

	// VersionValue is a semantic version according to semver.org spec 2.0.0.
	// Must not be longer than 64 characters.
	//
	// +optional
	// +oneOf=ValueType
	VersionValue *string `json:"version,omitempty"`
}

// DeviceCapacity defines consumable capacity of a device.
// +k8s:deepcopy-gen=true
type DeviceCapacity struct {
	// Quantity defines how much of a certain device capacity is available.
	Quantity resource.Quantity `json:"quantity,omitempty"`

	// potential future addition: fields which define how to "consume"
	// capacity (= share a single device between different consumers).
}

// Counter
// +k8s:deepcopy-gen=true
type Counter struct {
	// Value defines how much of a certain device counter is available.
	//
	// +required
	Value resource.Quantity `json:"value" protobuf:"bytes,1,rep,name=value"`
}
