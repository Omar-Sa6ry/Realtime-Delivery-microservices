import { TypeMetadataStorage } from '@nestjs/graphql';
import {
  GeneralResponse,
  BooleanResponse,
  IntResponse,
} from './general-response.type';

class OrderPayload {
  id!: string;
}

class DriverPayload {
  name!: string;
}

type AnyClass = new () => Record<string, any>;

function hydrateLazyMetadata() {
  const lazyStorage = (globalThis as any).GqlLazyMetadataStorageHost;
  lazyStorage?.load();
}

function objectTypeMetadata(target: any) {
  hydrateLazyMetadata();
  return TypeMetadataStorage.getObjectTypesMetadata().find(
    (meta) => meta.target === target,
  );
}

function fieldMetadata(target: any) {
  hydrateLazyMetadata();
  return TypeMetadataStorage.getClassFieldsByPredicate({ target }) as any[];
}

function fieldNames(target: any) {
  return fieldMetadata(target).map((field) => field.name);
}

describe('GeneralResponse', () => {
  it('returns a distinct abstract class per payload type', () => {
    const withOrders = GeneralResponse(OrderPayload) as AnyClass;
    const withDrivers = GeneralResponse(DriverPayload) as AnyClass;

    expect(withOrders).toBeDefined();
    expect(withOrders).not.toBe(withDrivers);
    expect(withOrders.name).toBe('GeneralResponseClass');
    expect(withOrders.prototype.constructor).toBe(withOrders);
  });

  it('creates instances with the documented default values', () => {
    const ResponseClass = GeneralResponse(OrderPayload) as AnyClass;
    const response = new ResponseClass();

    expect(response.message).toBe('Operation executed successfully');
    expect(response.success).toBe(true);
    expect(response.statusCode).toBe(200);
    expect(new Date(response.timeStamp).toISOString()).toBe(response.timeStamp);
    expect(response.data).toBeUndefined();
    expect(response.items).toBeUndefined();
  });

  it('accepts payload instances on data and items', () => {
    const ResponseClass = GeneralResponse(OrderPayload) as AnyClass;
    const response = new ResponseClass();

    response.data = { id: 'order-1' };
    response.items = [{ id: 'order-1' }, { id: 'order-2' }];

    expect(response.data).toEqual({ id: 'order-1' });
    expect(response.items).toHaveLength(2);
    expect(response.success).toBe(true);
  });

  it('registers an abstract object type named GeneralResponseClass', () => {
    const ResponseClass = GeneralResponse(OrderPayload);
    const metadata = objectTypeMetadata(ResponseClass);

    expect(metadata).toBeDefined();
    expect(metadata!.name).toBe('GeneralResponseClass');
    expect(metadata!.isAbstract).toBe(true);
  });

  it('registers the shareable directive on the generated class', () => {
    const ResponseClass = GeneralResponse(OrderPayload);
    hydrateLazyMetadata();

    const directives = TypeMetadataStorage.getInheritedClassDirectives(
      ResponseClass,
    );

    expect(directives).toEqual([
      expect.objectContaining({ sdl: '@shareable' }),
    ]);
  });

  it('declares the six response fields with their graphql types', () => {
    const ResponseClass = GeneralResponse(OrderPayload) as AnyClass;
    const fields = fieldMetadata(ResponseClass);
    const byName = (name: string) => fields.find((f) => f.name === name);

    expect(fieldNames(ResponseClass).sort()).toEqual([
      'data',
      'items',
      'message',
      'statusCode',
      'success',
      'timeStamp',
    ]);

    expect(byName('message')!.typeFn()).toBe(String);
    expect(byName('success')!.typeFn()).toBe(Boolean);
    expect((byName('statusCode')!.typeFn() as any).name).toBe('Int');
    expect(byName('data')!.typeFn()).toBe(OrderPayload);
    expect(byName('items')!.typeFn()).toBe(OrderPayload);
    expect(byName('items')!.options.isArray).toBe(true);
    expect(byName('items')!.options.arrayDepth).toBe(1);
    expect(byName('message')!.options.nullable).toBe(true);
    expect(byName('message')!.options.defaultValue).toBe(
      'Operation executed successfully',
    );
    expect(byName('success')!.options.defaultValue).toBe(true);
    expect(byName('statusCode')!.options.defaultValue).toBe(200);
  });

  it('points data and items at the supplied payload class', () => {
    const DriverClass = GeneralResponse(DriverPayload);
    const fields = fieldMetadata(DriverClass);
    const dataField = fields.find((f) => f.name === 'data');
    const itemsField = fields.find((f) => f.name === 'items');

    expect(dataField!.typeFn()).toBe(DriverPayload);
    expect(itemsField!.typeFn()).toBe(DriverPayload);
    expect(itemsField!.options.isArray).toBe(true);
  });

  it('resolves a graphql type for every declared field', () => {
    const resolved = [
      GeneralResponse(OrderPayload),
      BooleanResponse,
      IntResponse,
    ].flatMap((target) =>
      fieldMetadata(target).map((field) => ({
        name: field.name,
        type: field.typeFn(),
        options: field.options,
      })),
    );

    expect(resolved).toHaveLength(16);
    for (const field of resolved) {
      expect(field.type).toBeDefined();
      expect(field.options.nullable).toBe(true);
    }
    expect(resolved.filter((f) => f.options.isArray)).toHaveLength(1);
  });
});

describe('BooleanResponse', () => {
  it('defaults to a successful boolean response', () => {
    const response = new BooleanResponse();

    expect(response.message).toBe('Operation executed successfully');
    expect(response.success).toBe(true);
    expect(response.statusCode).toBe(200);
    expect(new Date(response.timeStamp).toISOString()).toBe(response.timeStamp);
    expect(response.data).toBeUndefined();
  });

  it('carries a boolean payload', () => {
    const response = new BooleanResponse();
    response.data = false;
    response.success = false;
    response.statusCode = 409;

    expect(response.data).toBe(false);
    expect(response.success).toBe(false);
    expect(response.statusCode).toBe(409);
  });

  it('is registered as an object type with shareable directive', () => {
    const metadata = objectTypeMetadata(BooleanResponse);

    expect(metadata).toBeDefined();
    expect(metadata!.name).toBe('BooleanResponse');
    expect(metadata!.isAbstract).toBeFalsy();

    hydrateLazyMetadata();
    expect(
      TypeMetadataStorage.getInheritedClassDirectives(BooleanResponse),
    ).toEqual([expect.objectContaining({ sdl: '@shareable' })]);
  });

  it('declares its fields with boolean data typing', () => {
    const fields = fieldMetadata(BooleanResponse);
    const byName = (name: string) => fields.find((f) => f.name === name);

    expect(fieldNames(BooleanResponse).sort()).toEqual([
      'data',
      'message',
      'statusCode',
      'success',
      'timeStamp',
    ]);
    expect(byName('data')!.typeFn()).toBe(Boolean);
    expect((byName('statusCode')!.typeFn() as any).name).toBe('Int');
    expect(byName('data')!.options.nullable).toBe(true);
  });
});

describe('IntResponse', () => {
  it('defaults to a successful integer response', () => {
    const response = new IntResponse();

    expect(response.message).toBe('Operation executed successfully');
    expect(response.success).toBe(true);
    expect(response.statusCode).toBe(200);
    expect(new Date(response.timeStamp).toISOString()).toBe(response.timeStamp);
    expect(response.data).toBeUndefined();
  });

  it('carries an integer payload', () => {
    const response = new IntResponse();
    response.data = 42;

    expect(response.data).toBe(42);
  });

  it('is registered as an object type with shareable directive', () => {
    const metadata = objectTypeMetadata(IntResponse);

    expect(metadata).toBeDefined();
    expect(metadata!.name).toBe('IntResponse');
    expect(metadata!.isAbstract).toBeFalsy();

    hydrateLazyMetadata();
    expect(
      TypeMetadataStorage.getInheritedClassDirectives(IntResponse),
    ).toEqual([expect.objectContaining({ sdl: '@shareable' })]);
  });

  it('declares its fields with integer data typing', () => {
    const fields = fieldMetadata(IntResponse);
    const byName = (name: string) => fields.find((f) => f.name === name);

    expect(fieldNames(IntResponse).sort()).toEqual([
      'data',
      'message',
      'statusCode',
      'success',
      'timeStamp',
    ]);
    expect((byName('data')!.typeFn() as any).name).toBe('Int');
    expect(byName('message')!.typeFn()).toBe(String);
    expect(byName('timeStamp')!.typeFn()).toBe(String);
  });
});

