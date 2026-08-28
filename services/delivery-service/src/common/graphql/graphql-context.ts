export interface GraphqlRequest {
  user?: { id?: string };
  headers?: Record<string, string | undefined>;
}

export interface GraphqlContext {
  req?: GraphqlRequest;
  language?: string;
}

