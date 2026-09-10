package graphql

import (
	"context"
	"fmt"
	"math"
	"time"

	gql "github.com/graph-gophers/graphql-go"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/commands"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/queries"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// RootResolver implements the GraphQL schema resolvers.
type RootResolver struct {
	driverRepo       ports.DriverRepository
	assignmentRepo   ports.AssignmentRepository
	dispatchService  *services.DispatchService
	registerHandler  *commands.RegisterDriverHandler
	blockHandler     *commands.BlockDriverHandler
	unblockHandler   *commands.UnblockDriverHandler
	rateDriverHandler *commands.RateDriverHandler
	getReviewsHandler *queries.GetDriverReviewsHandler
	eventPublisher   ports.EventPublisher
}

// NewRootResolver constructs a new RootResolver with all application dependencies.
func NewRootResolver(
	driverRepo ports.DriverRepository,
	assignmentRepo ports.AssignmentRepository,
	dispatchService *services.DispatchService,
	registerHandler *commands.RegisterDriverHandler,
	blockHandler *commands.BlockDriverHandler,
	unblockHandler *commands.UnblockDriverHandler,
	rateDriverHandler *commands.RateDriverHandler,
	getReviewsHandler *queries.GetDriverReviewsHandler,
	eventPublisher ports.EventPublisher,
) *RootResolver {
	return &RootResolver{
		driverRepo:        driverRepo,
		assignmentRepo:    assignmentRepo,
		dispatchService:   dispatchService,
		registerHandler:   registerHandler,
		blockHandler:      blockHandler,
		unblockHandler:    unblockHandler,
		rateDriverHandler: rateDriverHandler,
		getReviewsHandler: getReviewsHandler,
		eventPublisher:    eventPublisher,
	}
}
// PaginationInfoResolver
type PaginationInfoResolver struct {
	totalItems  int32
	currentPage int32
	nextPage    *int32
}

func (r *PaginationInfoResolver) TotalItems() int32 { return r.totalItems }
func (r *PaginationInfoResolver) CurrentPage() int32 { return r.currentPage }
func (r *PaginationInfoResolver) NextPage() *int32  { return r.nextPage }

// UserResolver for Apollo Federation reference representation
type UserResolver struct {
	id string
}

func (r *UserResolver) ID() gql.ID { return gql.ID(r.id) }


// DriverResolver
type DriverResolver struct {
	driver *domain.Driver
}

func (r *DriverResolver) ID() gql.ID { return gql.ID(r.driver.ID) }
func (r *DriverResolver) UserId() string { return r.driver.UserID }
func (r *DriverResolver) User() *UserResolver {
	if r.driver.UserID != "" {
		return &UserResolver{id: r.driver.UserID}
	}
	return nil
}

func (r *DriverResolver) Status() string { return string(r.driver.Status) }
func (r *DriverResolver) VehicleType() *string {
	if r.driver.Vehicle.Type != "" {
		s := string(r.driver.Vehicle.Type)
		return &s
	}
	return nil
}
func (r *DriverResolver) PlateNumber() *string {
	if r.driver.Vehicle.PlateNumber != "" {
		return &r.driver.Vehicle.PlateNumber
	}
	return nil
}
func (r *DriverResolver) CapacityKg() *int32 {
	c := int32(r.driver.Vehicle.CapacityKg)
	return &c
}
func (r *DriverResolver) Capabilities() *[]string {
	if r.driver.Capabilities != nil {
		caps := r.driver.Capabilities
		return &caps
	}
	empty := []string{}
	return &empty
}
func (r *DriverResolver) ServiceArea() *string {
	if r.driver.ServiceArea != "" {
		return &r.driver.ServiceArea
	}
	return nil
}
func (r *DriverResolver) Rating() *float64 {
	return &r.driver.Rating
}
func (r *DriverResolver) IsBlocked() bool {
	return r.driver.IsBlocked
}
func (r *DriverResolver) CreatedAt() *string {
	if !r.driver.CreatedAt.IsZero() {
		t := r.driver.CreatedAt.Format(time.RFC3339)
		return &t
	}
	return nil
}
func (r *DriverResolver) UpdatedAt() *string {
	if !r.driver.UpdatedAt.IsZero() {
		t := r.driver.UpdatedAt.Format(time.RFC3339)
		return &t
	}
	return nil
}

// DriverResponseResolver
type DriverResponseResolver struct {
	success    bool
	statusCode int32
	message    string
	timeStamp  string
	data       *domain.Driver
}

func (r *DriverResponseResolver) Success() bool              { return r.success }
func (r *DriverResponseResolver) StatusCode() int32          { return r.statusCode }
func (r *DriverResponseResolver) Message() string            { return r.message }
func (r *DriverResponseResolver) TimeStamp() string          { return r.timeStamp }
func (r *DriverResponseResolver) Data() *DriverResolver {
	if r.data == nil {
		return nil
	}
	return &DriverResolver{driver: r.data}
}

// AssignmentResolver
type AssignmentResolver struct {
	assignment *domain.Assignment
}

func (r *AssignmentResolver) ID() gql.ID { return gql.ID(r.assignment.ID) }
func (r *AssignmentResolver) DeliveryId() string { return r.assignment.DeliveryID }
func (r *AssignmentResolver) DriverId() string { return r.assignment.DriverID }
func (r *AssignmentResolver) Status() string { return string(r.assignment.Status) }
func (r *AssignmentResolver) AttemptNumber() *int32 {
	a := int32(r.assignment.AttemptNumber)
	return &a
}
func (r *AssignmentResolver) OfferedAt() *string {
	if !r.assignment.OfferedAt.IsZero() {
		t := r.assignment.OfferedAt.Format(time.RFC3339)
		return &t
	}
	return nil
}
func (r *AssignmentResolver) ExpiresAt() *string {
	if !r.assignment.ExpiresAt.IsZero() {
		t := r.assignment.ExpiresAt.Format(time.RFC3339)
		return &t
	}
	return nil
}
func (r *AssignmentResolver) AcceptedAt() *string {
	if r.assignment.AcceptedAt != nil && !r.assignment.AcceptedAt.IsZero() {
		t := r.assignment.AcceptedAt.Format(time.RFC3339)
		return &t
	}
	return nil
}
func (r *AssignmentResolver) RejectedAt() *string {
	if r.assignment.RejectedAt != nil && !r.assignment.RejectedAt.IsZero() {
		t := r.assignment.RejectedAt.Format(time.RFC3339)
		return &t
	}
	return nil
}
func (r *AssignmentResolver) CompletedAt() *string {
	if r.assignment.CompletedAt != nil && !r.assignment.CompletedAt.IsZero() {
		t := r.assignment.CompletedAt.Format(time.RFC3339)
		return &t
	}
	return nil
}
func (r *AssignmentResolver) CreatedAt() *string {
	if !r.assignment.CreatedAt.IsZero() {
		t := r.assignment.CreatedAt.Format(time.RFC3339)
		return &t
	}
	return nil
}
func (r *AssignmentResolver) UpdatedAt() *string {
	if !r.assignment.UpdatedAt.IsZero() {
		t := r.assignment.UpdatedAt.Format(time.RFC3339)
		return &t
	}
	return nil
}

// AssignmentResponseResolver
type AssignmentResponseResolver struct {
	success    bool
	statusCode int32
	message    string
	timeStamp  string
	data       *domain.Assignment
}

func (r *AssignmentResponseResolver) Success() bool                  { return r.success }
func (r *AssignmentResponseResolver) StatusCode() int32              { return r.statusCode }
func (r *AssignmentResponseResolver) Message() string                { return r.message }
func (r *AssignmentResponseResolver) TimeStamp() string              { return r.timeStamp }
func (r *AssignmentResponseResolver) Data() *AssignmentResolver {
	if r.data == nil {
		return nil
	}
	return &AssignmentResolver{assignment: r.data}
}

// DeliveryResolver
type DeliveryResolver struct {
	id string
}

func (r *DeliveryResolver) ID() gql.ID { return gql.ID(r.id) }

// DriverStatusResolver
type DriverStatusData struct {
	Driver              *domain.Driver
	Status              string
	HasActiveAssignment bool
	ActiveDeliveryID    *string
	LastSeenAt          *string
}

type DriverStatusResolver struct {
	data DriverStatusData
}

func (r *DriverStatusResolver) Driver() *DriverResolver {
	if r.data.Driver == nil {
		return nil
	}
	return &DriverResolver{driver: r.data.Driver}
}
func (r *DriverStatusResolver) Status() string                 { return r.data.Status }
func (r *DriverStatusResolver) HasActiveAssignment() bool      { return r.data.HasActiveAssignment }
func (r *DriverStatusResolver) ActiveDelivery() *DeliveryResolver {
	if r.data.ActiveDeliveryID == nil {
		return nil
	}
	return &DeliveryResolver{id: *r.data.ActiveDeliveryID}
}
func (r *DriverStatusResolver) LastSeenAt() *string            { return r.data.LastSeenAt }

// DriverStatusResponseResolver
type DriverStatusResponseResolver struct {
	success    bool
	statusCode int32
	message    string
	timeStamp  string
	data       *DriverStatusData
}

func (r *DriverStatusResponseResolver) Success() bool                    { return r.success }
func (r *DriverStatusResponseResolver) StatusCode() int32                { return r.statusCode }
func (r *DriverStatusResponseResolver) Message() string                  { return r.message }
func (r *DriverStatusResponseResolver) TimeStamp() string                { return r.timeStamp }
func (r *DriverStatusResponseResolver) Data() *DriverStatusResolver {
	if r.data == nil {
		return nil
	}
	return &DriverStatusResolver{data: *r.data}
}

// NearbyDriverItemResolver
type NearbyDriverItemData struct {
	Driver         *domain.Driver
	DistanceMeters float64
	Status         string
	VehicleType    *string
	Latitude       float64
	Longitude      float64
	Rating         float64
	IsBlocked      bool
}

type NearbyDriverItemResolver struct {
	item NearbyDriverItemData
}

func (r *NearbyDriverItemResolver) DriverId() string {
	if r.item.Driver != nil {
		return r.item.Driver.ID
	}
	return ""
}
func (r *NearbyDriverItemResolver) Driver() (*DriverResolver, error) {
	if r.item.Driver == nil {
		return nil, nil
	}
	return &DriverResolver{driver: r.item.Driver}, nil
}
func (r *NearbyDriverItemResolver) DistanceMeters() float64 { return r.item.DistanceMeters }
func (r *NearbyDriverItemResolver) Status() string         { return r.item.Status }
func (r *NearbyDriverItemResolver) VehicleType() *string   { return r.item.VehicleType }
func (r *NearbyDriverItemResolver) Latitude() float64      { return r.item.Latitude }
func (r *NearbyDriverItemResolver) Longitude() float64     { return r.item.Longitude }
func (r *NearbyDriverItemResolver) Rating() float64        { return r.item.Rating }
func (r *NearbyDriverItemResolver) IsBlocked() bool        { return r.item.IsBlocked }

// NearbyDriversDataResolver
type NearbyDriversDataResolver struct {
	paginationInfo PaginationInfoResolver
	items          []*NearbyDriverItemResolver
}

func (r *NearbyDriversDataResolver) PaginationInfo() *PaginationInfoResolver {
	return &r.paginationInfo
}
func (r *NearbyDriversDataResolver) Items() []*NearbyDriverItemResolver {
	return r.items
}

// NearbyDriversResponseResolver
type NearbyDriversResponseResolver struct {
	success    bool
	statusCode int32
	message    string
	timeStamp  string
	data       *NearbyDriversDataResolver
}

func (r *NearbyDriversResponseResolver) Success() bool                        { return r.success }
func (r *NearbyDriversResponseResolver) StatusCode() int32                    { return r.statusCode }
func (r *NearbyDriversResponseResolver) Message() string                      { return r.message }
func (r *NearbyDriversResponseResolver) TimeStamp() string                    { return r.timeStamp }
func (r *NearbyDriversResponseResolver) Data() *NearbyDriversDataResolver { return r.data }

// ReviewResolver
type ReviewResolver struct {
	review *domain.Review
}

func (r *ReviewResolver) ID() gql.ID         { return gql.ID(r.review.ID) }
func (r *ReviewResolver) DriverId() string   { return r.review.DriverID }
func (r *ReviewResolver) UserId() string     { return r.review.UserID }
func (r *ReviewResolver) DeliveryId() string { return r.review.DeliveryID }

func (r *ReviewResolver) Driver(ctx context.Context) (*DriverResolver, error) {
	loaders := GetLoaders(ctx)
	if loaders == nil || loaders.DriverLoader == nil {
		return &DriverResolver{driver: &domain.Driver{ID: r.review.DriverID}}, nil
	}
	driver, err := loaders.DriverLoader.Load(ctx, r.review.DriverID)()
	if err != nil {
		return nil, err
	}
	return &DriverResolver{driver: driver}, nil
}

func (r *ReviewResolver) User() *UserResolver {
	return &UserResolver{id: r.review.UserID}
}

func (r *ReviewResolver) Delivery() *DeliveryResolver {
	return &DeliveryResolver{id: r.review.DeliveryID}
}

func (r *ReviewResolver) Rating() float64    { return r.review.Rating }
func (r *ReviewResolver) Comment() *string {
	if r.review.Comment != "" {
		return &r.review.Comment
	}
	return nil
}
func (r *ReviewResolver) CreatedAt() string {
	return r.review.CreatedAt.Format(time.RFC3339)
}

// ReviewResponseResolver
type ReviewResponseResolver struct {
	success    bool
	statusCode int32
	message    string
	timeStamp  string
	data       *domain.Review
}

func (r *ReviewResponseResolver) Success() bool              { return r.success }
func (r *ReviewResponseResolver) StatusCode() int32          { return r.statusCode }
func (r *ReviewResponseResolver) Message() string            { return r.message }
func (r *ReviewResponseResolver) TimeStamp() string          { return r.timeStamp }
func (r *ReviewResponseResolver) Data() *ReviewResolver {
	if r.data == nil {
		return nil
	}
	return &ReviewResolver{review: r.data}
}

// DriverReviewsDataResolver
type DriverReviewsDataResolver struct {
	paginationInfo PaginationInfoResolver
	items          []*ReviewResolver
	averageRating  float64
	totalReviews   int32
}

func (r *DriverReviewsDataResolver) PaginationInfo() *PaginationInfoResolver {
	return &r.paginationInfo
}
func (r *DriverReviewsDataResolver) Items() []*ReviewResolver { return r.items }
func (r *DriverReviewsDataResolver) AverageRating() float64   { return r.averageRating }
func (r *DriverReviewsDataResolver) TotalReviews() int32      { return r.totalReviews }

// DriverReviewsResponseResolver
type DriverReviewsResponseResolver struct {
	success    bool
	statusCode int32
	message    string
	timeStamp  string
	data       *DriverReviewsDataResolver
}

func (r *DriverReviewsResponseResolver) Success() bool                        { return r.success }
func (r *DriverReviewsResponseResolver) StatusCode() int32                    { return r.statusCode }
func (r *DriverReviewsResponseResolver) Message() string                      { return r.message }
func (r *DriverReviewsResponseResolver) TimeStamp() string                    { return r.timeStamp }
func (r *DriverReviewsResponseResolver) Data() *DriverReviewsDataResolver { return r.data }

// DriverServiceInfoResolver
type DriverServiceInfoData struct {
	Name    string
	Version string
	Status  string
}

type DriverServiceInfoResolver struct {
	data DriverServiceInfoData
}

func (r *DriverServiceInfoResolver) Name() string    { return r.data.Name }
func (r *DriverServiceInfoResolver) Version() string { return r.data.Version }
func (r *DriverServiceInfoResolver) Status() string  { return r.data.Status }

type DriverServiceInfoResponseResolver struct {
	success    bool
	statusCode int32
	message    string
	timeStamp  string
	data       *DriverServiceInfoData
}

func (r *DriverServiceInfoResponseResolver) Success() bool                        { return r.success }
func (r *DriverServiceInfoResponseResolver) StatusCode() int32                    { return r.statusCode }
func (r *DriverServiceInfoResponseResolver) Message() string                      { return r.message }
func (r *DriverServiceInfoResponseResolver) TimeStamp() string                    { return r.timeStamp }
func (r *DriverServiceInfoResponseResolver) Data() *DriverServiceInfoResolver {
	if r.data == nil {
		return nil
	}
	return &DriverServiceInfoResolver{data: *r.data}
}

// DispatchAttemptItemResolver
type DispatchAttemptItemData struct {
	ID            string
	DeliveryID    string
	DriverID      string
	DistanceMeters float64
	AttemptNumber int32
	Result        string
	Reason        *string
	CreatedAt     string
}

type DispatchAttemptItemResolver struct {
	item DispatchAttemptItemData
}

func (r *DispatchAttemptItemResolver) ID() gql.ID               { return gql.ID(r.item.ID) }
func (r *DispatchAttemptItemResolver) DeliveryId() string       { return r.item.DeliveryID }
func (r *DispatchAttemptItemResolver) DriverId() string         { return r.item.DriverID }
func (r *DispatchAttemptItemResolver) Delivery() *DeliveryResolver {
	return &DeliveryResolver{id: r.item.DeliveryID}
}
func (r *DispatchAttemptItemResolver) Driver(ctx context.Context) (*DriverResolver, error) {
	loaders := GetLoaders(ctx)
	if loaders == nil || loaders.DriverLoader == nil {
		return &DriverResolver{driver: &domain.Driver{ID: r.item.DriverID}}, nil
	}
	driver, err := loaders.DriverLoader.Load(ctx, r.item.DriverID)()
	if err != nil {
		return nil, err
	}
	return &DriverResolver{driver: driver}, nil
}
func (r *DispatchAttemptItemResolver) DistanceMeters() float64 { return r.item.DistanceMeters }
func (r *DispatchAttemptItemResolver) AttemptNumber() int32    { return r.item.AttemptNumber }
func (r *DispatchAttemptItemResolver) Result() string           { return r.item.Result }
func (r *DispatchAttemptItemResolver) Reason() *string          { return r.item.Reason }
func (r *DispatchAttemptItemResolver) CreatedAt() string        { return r.item.CreatedAt }

// DispatchAttemptsDataResolver
type DispatchAttemptsDataResolver struct {
	paginationInfo PaginationInfoResolver
	items          []*DispatchAttemptItemResolver
}

func (r *DispatchAttemptsDataResolver) PaginationInfo() *PaginationInfoResolver {
	return &r.paginationInfo
}
func (r *DispatchAttemptsDataResolver) Items() []*DispatchAttemptItemResolver {
	return r.items
}

// DispatchAttemptsResponseResolver
type DispatchAttemptsResponseResolver struct {
	success    bool
	statusCode int32
	message    string
	timeStamp  string
	data       *DispatchAttemptsDataResolver
}

func (r *DispatchAttemptsResponseResolver) Success() bool                            { return r.success }
func (r *DispatchAttemptsResponseResolver) StatusCode() int32                        { return r.statusCode }
func (r *DispatchAttemptsResponseResolver) Message() string                          { return r.message }
func (r *DispatchAttemptsResponseResolver) TimeStamp() string                        { return r.timeStamp }
func (r *DispatchAttemptsResponseResolver) Data() *DispatchAttemptsDataResolver { return r.data }

// _Service
type ServiceResolver struct {
	sdl string
}

func (r *ServiceResolver) Sdl() string { return r.sdl }


func (r *AssignmentResolver) Delivery() *DeliveryResolver {
	return &DeliveryResolver{id: r.assignment.DeliveryID}
}

func (r *AssignmentResolver) Driver(ctx context.Context) (*DriverResolver, error) {
	loaders := GetLoaders(ctx)
	if loaders == nil || loaders.DriverLoader == nil {
		return &DriverResolver{driver: &domain.Driver{ID: r.assignment.DriverID}}, nil
	}
	driver, err := loaders.DriverLoader.Load(ctx, r.assignment.DriverID)()
	if err != nil {
		return nil, err
	}
	return &DriverResolver{driver: driver}, nil
}

func (r *RootResolver) Service() *ServiceResolver {
	return &ServiceResolver{sdl: DriverSubgraphSDL}
}

func (r *RootResolver) DriverServiceInfo(ctx context.Context) *DriverServiceInfoResponseResolver {
	now := time.Now().UTC().Format(time.RFC3339)
	return &DriverServiceInfoResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Driver service is operational",
		timeStamp:  now,
		data: &DriverServiceInfoData{
			Name:    "driver-service",
			Version: "1.0.0",
			Status:  "healthy",
		},
	}
}

func (r *RootResolver) Driver(ctx context.Context, args struct{ Id gql.ID }) (*DriverResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	driver, err := r.driverRepo.FindByID(ctx, string(args.Id))
	if err != nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	if driver == nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 404,
			message:    "Driver not found",
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	return &DriverResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Driver fetched successfully",
		timeStamp:  now,
		data:       driver,
	}, nil
}

func (r *RootResolver) MyDriverProfile(ctx context.Context) (*DriverResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	// Extract user from context if available, or return error/not found
	userID, ok := ctx.Value("userID").(string)
	if !ok || userID == "" {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 401,
			message:    "Unauthorized: missing user context",
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	driver, err := r.driverRepo.FindByUserID(ctx, userID)
	if err != nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	if driver == nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 404,
			message:    "No driver profile found for this user",
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	return &DriverResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Driver profile fetched successfully",
		timeStamp:  now,
		data:       driver,
	}, nil
}

func (r *RootResolver) DriverActiveAssignment(ctx context.Context, args struct{ DriverId gql.ID }) (*AssignmentResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	assignment, err := r.assignmentRepo.FindActiveByDriver(ctx, string(args.DriverId))
	if err != nil {
		return &AssignmentResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	if assignment == nil {
		return &AssignmentResponseResolver{
			success:    true,
			statusCode: 200,
			message:    "No active assignment found for driver",
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	return &AssignmentResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Active assignment fetched successfully",
		timeStamp:  now,
		data:       assignment,
	}, nil
}

func (r *RootResolver) DriverStatus(ctx context.Context, args struct{ DriverId gql.ID }) (*DriverStatusResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	driver, err := r.driverRepo.FindByID(ctx, string(args.DriverId))
	if err != nil {
		return &DriverStatusResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	if driver == nil {
		return &DriverStatusResponseResolver{
			success:    false,
			statusCode: 404,
			message:    "Driver not found",
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	activeAssignment, _ := r.assignmentRepo.FindActiveByDriver(ctx, string(args.DriverId))
	hasActive := activeAssignment != nil
	var deliveryID *string
	if hasActive {
		deliveryID = &activeAssignment.DeliveryID
	}
	var lastSeen *string
	if !driver.UpdatedAt.IsZero() {
		ls := driver.UpdatedAt.Format(time.RFC3339)
		lastSeen = &ls
	}

	return &DriverStatusResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Driver status fetched successfully",
		timeStamp:  now,
		data: &DriverStatusData{
			Driver:              driver,
			Status:              string(driver.Status),
			HasActiveAssignment: hasActive,
			ActiveDeliveryID:    deliveryID,
			LastSeenAt:          lastSeen,
		},
	}, nil
}

type NearbyDriversInput struct {
	Latitude    float64
	Longitude   float64
	RadiusKm    float64
	VehicleType *string
	Limit       *int32
}

func (r *RootResolver) NearbyDrivers(ctx context.Context, args struct{ Input NearbyDriversInput }) (*NearbyDriversResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var vType domain.VehicleType
	if args.Input.VehicleType != nil {
		vType = domain.VehicleType(*args.Input.VehicleType)
	}

	var candidates []domain.Candidate
	var err error
	if r.dispatchService != nil {
		candidates, err = r.dispatchService.FindAvailableDrivers(
			ctx,
			args.Input.Latitude,
			args.Input.Longitude,
			args.Input.RadiusKm,
			vType,
			"graphql-nearby-query",
		)
	} else {
		drivers, errRepo := r.driverRepo.FindAvailableByLocation(
			ctx,
			args.Input.Latitude,
			args.Input.Longitude,
			args.Input.RadiusKm,
			vType,
		)
		if errRepo == nil {
			for _, d := range drivers {
				candidates = append(candidates, domain.Candidate{
					DriverID:       d.ID,
					DistanceMeters: 0,
					VehicleType:    d.Vehicle.Type,
					Status:         d.Status,
				})
			}
		}
		err = errRepo
	}

	if err != nil {
		return &NearbyDriversResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	limit := 50
	if args.Input.Limit != nil && *args.Input.Limit > 0 {
		limit = int(*args.Input.Limit)
	}

	var items []*NearbyDriverItemResolver
	loaders := GetLoaders(ctx)

	// Collect IDs to fetch in bulk if loaders is available
	var driverIDs []string
	var driverCandidates []domain.Candidate
	for i, c := range candidates {
		if i >= limit {
			break
		}
		driverIDs = append(driverIDs, c.DriverID)
		driverCandidates = append(driverCandidates, c)
	}

	var loadedDrivers []*domain.Driver
	if loaders != nil && loaders.DriverLoader != nil {
		loadedDrivers, _ = loaders.DriverLoader.LoadMany(ctx, driverIDs)()
	}

	for i, c := range driverCandidates {
		vt := string(c.VehicleType)
		
		var driver *domain.Driver
		if i < len(loadedDrivers) && loadedDrivers[i] != nil {
			driver = loadedDrivers[i]
		} else {
			// Fallback if no loader or loaded failed
			driver, _ = r.driverRepo.FindByID(ctx, c.DriverID)
		}
		
		rating := 0.0
		isBlocked := false
		if driver != nil {
			rating = driver.Rating
			isBlocked = driver.IsBlocked
		}
		items = append(items, &NearbyDriverItemResolver{
			item: NearbyDriverItemData{
				Driver:         driver,
				DistanceMeters: c.DistanceMeters,
				Status:         string(c.Status),
				VehicleType:    &vt,
				Latitude:       args.Input.Latitude,
				Longitude:      args.Input.Longitude,
				Rating:         rating,
				IsBlocked:      isBlocked,
			},
		})
	}

	return &NearbyDriversResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Nearby drivers fetched successfully",
		timeStamp:  now,
		data: &NearbyDriversDataResolver{
			paginationInfo: PaginationInfoResolver{
				totalItems:  int32(len(candidates)),
				currentPage: 1,
				nextPage:    nil,
			},
			items: items,
		},
	}, nil
}

func (r *RootResolver) Assignment(ctx context.Context, args struct{ Id gql.ID }) (*AssignmentResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	assignment, err := r.assignmentRepo.FindByID(ctx, string(args.Id))
	if err != nil {
		return &AssignmentResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	if assignment == nil {
		return &AssignmentResponseResolver{
			success:    false,
			statusCode: 404,
			message:    "Assignment not found",
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	return &AssignmentResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Assignment fetched successfully",
		timeStamp:  now,
		data:       assignment,
	}, nil
}

func (r *RootResolver) DispatchAttempts(ctx context.Context, args struct{ DeliveryId gql.ID }) (*DispatchAttemptsResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	// Query assignments associated with this delivery ID
	assignment, err := r.assignmentRepo.FindByDeliveryID(ctx, string(args.DeliveryId))
	if err != nil {
		return &DispatchAttemptsResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	var items []*DispatchAttemptItemResolver
	if assignment != nil {
		reason := ""
		items = append(items, &DispatchAttemptItemResolver{
			item: DispatchAttemptItemData{
				ID:            assignment.ID,
				DeliveryID:    assignment.DeliveryID,
				DriverID:      assignment.DriverID,
				DistanceMeters: 0,
				AttemptNumber: int32(assignment.AttemptNumber),
				Result:        string(assignment.Status),
				Reason:        &reason,
				CreatedAt:     assignment.CreatedAt.Format(time.RFC3339),
			},
		})
	}

	return &DispatchAttemptsResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Dispatch attempts fetched successfully",
		timeStamp:  now,
		data: &DispatchAttemptsDataResolver{
			paginationInfo: PaginationInfoResolver{
				totalItems:  int32(len(items)),
				currentPage: 1,
				nextPage:    nil,
			},
			items: items,
		},
	}, nil
}

func (r *RootResolver) DriverReviews(ctx context.Context, args struct {
	DriverId gql.ID
	Page     *int32
	Limit    *int32
}) (*DriverReviewsResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	page := 1
	if args.Page != nil && *args.Page > 0 {
		page = int(*args.Page)
	}
	limit := 10
	if args.Limit != nil && *args.Limit > 0 {
		limit = int(*args.Limit)
	}

	res, err := r.getReviewsHandler.Execute(ctx, queries.GetDriverReviewsQuery{
		DriverID: string(args.DriverId),
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		return &DriverReviewsResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	items := make([]*ReviewResolver, len(res.Reviews))
	for i, rev := range res.Reviews {
		items[i] = &ReviewResolver{review: rev}
	}

	var nextPage *int32
	if int64(res.CurrentPage*res.Limit) < res.TotalItems {
		np := int32(res.CurrentPage + 1)
		nextPage = &np
	}

	return &DriverReviewsResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Driver reviews fetched successfully",
		timeStamp:  now,
		data: &DriverReviewsDataResolver{
			paginationInfo: PaginationInfoResolver{
				totalItems:  int32(res.TotalItems),
				currentPage: int32(res.CurrentPage),
				nextPage:    nextPage,
			},
			items:         items,
			averageRating: res.AverageRating,
			totalReviews:  int32(res.TotalReviews),
		},
	}, nil
}

func (r *RootResolver) authorizeActiveDriver(ctx context.Context) (*domain.Driver, error) {
	userID, _ := ctx.Value("userID").(string)
	if userID == "" {
		return nil, fmt.Errorf("Unauthorized")
	}
	driver, err := r.driverRepo.FindByUserID(ctx, userID)
	if err != nil || driver == nil {
		return nil, fmt.Errorf("Driver profile not found")
	}
	if driver.IsBlocked || driver.Status == domain.DriverStatusSuspended {
		return nil, fmt.Errorf("Driver is blocked or suspended and cannot perform this action")
	}
	return driver, nil
}

func (r *RootResolver) GoOnline(ctx context.Context, args struct{ IdempotencyKey string }) (*DriverResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	driver, err := r.authorizeActiveDriver(ctx)
	if err != nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 403,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	cmd := commands.NewGoOnlineCommand(driver.ID, r.driverRepo, r.eventPublisher)
	if err := cmd.Execute(ctx); err != nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 400,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	updatedDriver, _ := r.driverRepo.FindByID(ctx, driver.ID)
	return &DriverResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Driver is now online and available",
		timeStamp:  now,
		data:       updatedDriver,
	}, nil
}

func (r *RootResolver) GoOffline(ctx context.Context, args struct{ IdempotencyKey string }) (*DriverResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	driver, err := r.authorizeActiveDriver(ctx)
	if err != nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 403,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	cmd := commands.NewGoOfflineCommand(driver.ID, r.driverRepo, r.eventPublisher)
	if err := cmd.Execute(ctx); err != nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 400,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	updatedDriver, _ := r.driverRepo.FindByID(ctx, driver.ID)
	return &DriverResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Driver is now offline",
		timeStamp:  now,
		data:       updatedDriver,
	}, nil
}

func (r *RootResolver) AcceptAssignment(ctx context.Context, args struct {
	AssignmentId   gql.ID
	IdempotencyKey string
}) (*AssignmentResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	driver, err := r.authorizeActiveDriver(ctx)
	if err != nil {
		return &AssignmentResponseResolver{
			success:    false,
			statusCode: 403,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	assignment, err := r.assignmentRepo.FindByID(ctx, string(args.AssignmentId))
	if err != nil || assignment == nil || assignment.DriverID != driver.ID {
		return &AssignmentResponseResolver{
			success:    false,
			statusCode: 404,
			message:    "Assignment not found",
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	cmd := commands.NewAcceptAssignmentCommand(assignment.ID, assignment.DriverID, r.assignmentRepo, r.eventPublisher)
	if err := cmd.Execute(ctx); err != nil {
		return &AssignmentResponseResolver{
			success:    false,
			statusCode: 400,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	updatedAssignment, _ := r.assignmentRepo.FindByID(ctx, assignment.ID)
	return &AssignmentResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Assignment accepted successfully",
		timeStamp:  now,
		data:       updatedAssignment,
	}, nil
}

func (r *RootResolver) RejectAssignment(ctx context.Context, args struct {
	AssignmentId   gql.ID
	Reason         *string
	IdempotencyKey string
}) (*AssignmentResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	driver, err := r.authorizeActiveDriver(ctx)
	if err != nil {
		return &AssignmentResponseResolver{
			success:    false,
			statusCode: 403,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	assignment, err := r.assignmentRepo.FindByID(ctx, string(args.AssignmentId))
	if err != nil || assignment == nil || assignment.DriverID != driver.ID {
		return &AssignmentResponseResolver{
			success:    false,
			statusCode: 404,
			message:    "Assignment not found",
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	reason := ""
	if args.Reason != nil {
		reason = *args.Reason
	}
	cmd := commands.NewRejectAssignmentCommand(assignment.ID, assignment.DriverID, reason, r.assignmentRepo, r.eventPublisher)
	if err := cmd.Execute(ctx); err != nil {
		return &AssignmentResponseResolver{
			success:    false,
			statusCode: 400,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	updatedAssignment, _ := r.assignmentRepo.FindByID(ctx, assignment.ID)
	return &AssignmentResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Assignment rejected successfully",
		timeStamp:  now,
		data:       updatedAssignment,
	}, nil
}

type RegisterDriverInput struct {
	UserId       string
	VehicleType  string
	PlateNumber  string
	CapacityKg   int32
	Capabilities *[]string
	ServiceArea  *string
}

func (r *RootResolver) RegisterDriver(ctx context.Context, args struct{ Input RegisterDriverInput }) (*DriverResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var capabilities []string
	if args.Input.Capabilities != nil {
		capabilities = *args.Input.Capabilities
	}
	serviceArea := ""
	if args.Input.ServiceArea != nil {
		serviceArea = *args.Input.ServiceArea
	}

	cmd := commands.RegisterDriverCommand{
		UserID:       args.Input.UserId,
		VehicleType:  domain.VehicleType(args.Input.VehicleType),
		PlateNumber:  args.Input.PlateNumber,
		CapacityKg:   int64(args.Input.CapacityKg),
		Capabilities: capabilities,
		ServiceArea:  serviceArea,
	}

	driver, err := r.registerHandler.Execute(ctx, cmd)
	if err != nil {
		statusCode := int32(500)
		if domainErr, ok := err.(*domain.Error); ok && domainErr.Code == domain.ErrDriverAlreadyExists.Code {
			statusCode = 400
		}
		return &DriverResponseResolver{
			success:    false,
			statusCode: statusCode,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	return &DriverResponseResolver{
		success:    true,
		statusCode: 201,
		message:    "Driver registered successfully",
		timeStamp:  now,
		data:       driver,
	}, nil
}

type UpdateDriverProfileInput struct {
	DriverId    gql.ID
	VehicleType *string
	PlateNumber *string
	CapacityKg  *int32
	ServiceArea *string
}

func (r *RootResolver) UpdateDriverProfile(ctx context.Context, args struct{ Input UpdateDriverProfileInput }) (*DriverResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	role, _ := ctx.Value("role").(string)
	if role != "admin" {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 403,
			message:    "Forbidden: Only admin can update driver profile",
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	driver, err := r.driverRepo.FindByID(ctx, string(args.Input.DriverId))
	if err != nil || driver == nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 404,
			message:    "Driver profile not found",
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	if args.Input.VehicleType != nil {
		driver.Vehicle.Type = domain.VehicleType(*args.Input.VehicleType)
	}
	if args.Input.PlateNumber != nil {
		driver.Vehicle.PlateNumber = *args.Input.PlateNumber
	}
	if args.Input.CapacityKg != nil {
		driver.Vehicle.CapacityKg = int64(*args.Input.CapacityKg)
	}
	if args.Input.ServiceArea != nil {
		driver.ServiceArea = *args.Input.ServiceArea
	}
	driver.UpdatedAt = time.Now().UTC()

	if err := r.driverRepo.Save(ctx, driver); err != nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	return &DriverResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Driver profile updated successfully",
		timeStamp:  now,
		data:       driver,
	}, nil
}

func (r *RootResolver) SuspendDriver(ctx context.Context, args struct {
	DriverId gql.ID
	Reason   string
}) (*DriverResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	driver, err := r.driverRepo.FindByID(ctx, string(args.DriverId))
	if err != nil || driver == nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 404,
			message:    "Driver not found",
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	driver.Status = domain.DriverStatusSuspended
	driver.UpdatedAt = time.Now().UTC()
	if err := r.driverRepo.Save(ctx, driver); err != nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	return &DriverResponseResolver{
		success:    true,
		statusCode: 200,
		message:    fmt.Sprintf("Driver suspended successfully: %s", args.Reason),
		timeStamp:  now,
		data:       driver,
	}, nil
}

func (r *RootResolver) ActivateDriver(ctx context.Context, args struct{ DriverId gql.ID }) (*DriverResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	driver, err := r.driverRepo.FindByID(ctx, string(args.DriverId))
	if err != nil || driver == nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 404,
			message:    "Driver not found",
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	driver.Status = domain.DriverStatusAvailable
	driver.UpdatedAt = time.Now().UTC()
	if err := r.driverRepo.Save(ctx, driver); err != nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	return &DriverResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Driver activated successfully",
		timeStamp:  now,
		data:       driver,
	}, nil
}

func (r *RootResolver) BlockDriver(ctx context.Context, args struct {
	DriverId gql.ID
	Reason   *string
}) (*DriverResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	role, _ := ctx.Value("role").(string)
	if role != "admin" {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 403,
			message:    "Forbidden: Only admin can block driver",
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	reason := ""
	if args.Reason != nil {
		reason = *args.Reason
	}
	driver, err := r.blockHandler.Execute(ctx, commands.BlockDriverCommand{
		DriverID: string(args.DriverId),
		Reason:   reason,
	})
	if err != nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	return &DriverResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Driver blocked successfully",
		timeStamp:  now,
		data:       driver,
	}, nil
}

func (r *RootResolver) UnblockDriver(ctx context.Context, args struct {
	DriverId gql.ID
	Reason   *string
}) (*DriverResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	role, _ := ctx.Value("role").(string)
	if role != "admin" {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 403,
			message:    "Forbidden: Only admin can unblock driver",
			timeStamp:  now,
			data:       nil,
		}, nil
	}
	reason := ""
	if args.Reason != nil {
		reason = *args.Reason
	}
	driver, err := r.unblockHandler.Execute(ctx, commands.UnblockDriverCommand{
		DriverID: string(args.DriverId),
		Reason:   reason,
	})
	if err != nil {
		return &DriverResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	return &DriverResponseResolver{
		success:    true,
		statusCode: 200,
		message:    "Driver unblocked successfully",
		timeStamp:  now,
		data:       driver,
	}, nil
}

type RateDriverInput struct {
	DriverId   gql.ID
	DeliveryId gql.ID
	Rating     float64
	Comment    *string
}

func (r *RootResolver) RateDriver(ctx context.Context, args struct{ Input RateDriverInput }) (*ReviewResponseResolver, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	userID, _ := ctx.Value("userID").(string)
	if userID == "" {
		userID = "user-client"
	}
	comment := ""
	if args.Input.Comment != nil {
		comment = *args.Input.Comment
	}

	// Validate rating bounds
	if args.Input.Rating < 1.0 || args.Input.Rating > 5.0 || math.IsNaN(args.Input.Rating) {
		return &ReviewResponseResolver{
			success:    false,
			statusCode: 400,
			message:    "Rating must be between 1.0 and 5.0",
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	review, err := r.rateDriverHandler.Execute(ctx, commands.RateDriverCommand{
		DriverID:   string(args.Input.DriverId),
		UserID:     userID,
		DeliveryID: string(args.Input.DeliveryId),
		Rating:     args.Input.Rating,
		Comment:    comment,
	})
	if err != nil {
		return &ReviewResponseResolver{
			success:    false,
			statusCode: 500,
			message:    err.Error(),
			timeStamp:  now,
			data:       nil,
		}, nil
	}

	return &ReviewResponseResolver{
		success:    true,
		statusCode: 201,
		message:    "Driver rated successfully",
		timeStamp:  now,
		data:       review,
	}, nil
}
