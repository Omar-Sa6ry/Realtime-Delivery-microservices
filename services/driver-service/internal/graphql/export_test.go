package graphql

// Test-only constructors that expose the parts of the resolvers no query can
// reach on its own. This file only exists in test builds, keeping the external
// graphql_test package able to call the unexported-field types directly.
//
// DriverStatusData.Driver is always non-nil on the success path of the
// driverStatus query, and DriverServiceInfoResponseResolver.data is always
// populated by RootResolver.DriverServiceInfo, so their nil branches are
// purely defensive. The external tests build both shapes here to cover them.

// NewDriverStatusResolverForTest wraps raw status data in a resolver.
func NewDriverStatusResolverForTest(data DriverStatusData) *DriverStatusResolver {
	return &DriverStatusResolver{data: data}
}

// NewEmptyDriverServiceInfoResponseForTest builds a response whose payload is
// nil, the state no root resolver ever produces.
func NewEmptyDriverServiceInfoResponseForTest() *DriverServiceInfoResponseResolver {
	return &DriverServiceInfoResponseResolver{}
}
