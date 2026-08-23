export class ResponseFormatter {
  static formatSuccess(data: any): {
    success: boolean;
    statusCode: number;
    message: string;
    timeStamp: string;
    data: any;
    items?: any[];
    pagination?: any;
  } {
    const isArray = Array.isArray(data);
    const isEnvelope = data && typeof data === 'object' && !isArray && 'success' in data && 'statusCode' in data;

    let items: any[] = [];
    if (Array.isArray(data?.items)) {
      items = data.items;
    } else if (Array.isArray(data?.data?.items)) {
      items = data.data.items;
    } else if (isArray) {
      items = data;
    }

    return {
      success: true,
      statusCode: isEnvelope ? data.statusCode : 200,
      message: isEnvelope ? data.message : (data?.message || 'Request successful'),
      timeStamp: new Date().toISOString(),
      data: isEnvelope ? data.data : (data?.data !== undefined ? data.data : (data ?? null)),
      items: items.length > 0 ? items : undefined,
      pagination: data?.pagination,
    };
  }

  static formatError(error: any): {
    success: boolean;
    statusCode: number;
    message: string;
    timeStamp: string;
    error?: string;
  } {
    const message = error?.errors?.map((err: any) => err?.message)?.join(', ') ||
      error?.response?.message ||
      error?.message ||
      'An unexpected error occurred';

    const statusCode = error?.response?.statusCode || error?.status || error?.statusCode || 500;

    return {
      success: false,
      statusCode,
      message: Array.isArray(message) ? message[0] : message,
      timeStamp: new Date().toISOString(),
      error: error?.response?.error || 'Unknown error',
    };
  }
}