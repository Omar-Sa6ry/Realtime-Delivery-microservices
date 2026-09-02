export enum SearchIndex {
  DELIVERIES = 'deliveries',
  DRIVERS = 'drivers',
  MEDIA = 'media',
}

export enum SearchKafkaTopics {
  DELIVERY_CREATED = 'delivery.created',
  DELIVERY_DRIVER_ASSIGNED = 'delivery.driver.assigned',
  DELIVERY_DRIVER_ACCEPTED = 'delivery.driver.accepted',
  DELIVERY_PICKED_UP = 'delivery.picked_up',
  DELIVERY_IN_TRANSIT = 'delivery.in_transit',
  DELIVERY_COMPLETED = 'delivery.completed',
  DELIVERY_CANCELLED = 'delivery.cancelled',
  DELIVERY_DELETED = 'delivery.deleted',
  DRIVER_CREATED = 'driver.created',
  DRIVER_UPDATED = 'driver.updated',
  DRIVER_DELETED = 'driver.deleted',
  MEDIA_READY = 'media.ready',
  MEDIA_DELETED = 'media.deleted',
  DLQ = 'search.dlq',
  QUERY_COMPLETED = 'search.query.completed',
  REINDEX_COMPLETED = 'search.reindex.completed',
}

export const SEARCH_CONSUMER_GROUP_ID = 'search-service';

export const SearchMaxPageSize = 100;
export const SearchDefaultPageSize = 10;
export const SearchMaxQueryLength = 500;
export const SearchMaxFuzziness = 2;
export const SearchCacheTTLSeconds = 120;
export const SearchSuggestCacheTTL = 300;

export enum DriverStatus {
  AVAILABLE = 'AVAILABLE',
  BUSY = 'BUSY',
  OFFLINE = 'OFFLINE',
}

export enum VehicleType {
  CAR = 'CAR',
  MOTORCYCLE = 'MOTORCYCLE',
  TRUCK = 'TRUCK',
  BICYCLE = 'BICYCLE',
}
