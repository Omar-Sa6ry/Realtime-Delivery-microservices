/**
 * Jest (CJS) cannot load `uuid@14`, which ships ESM-only builds. The gateway
 * only uses `v4`, which is exactly `crypto.randomUUID()` — so this shim keeps
 * the real behaviour (RFC9522 v4 UUIDs) while staying loadable under Jest.
 */
import { randomUUID } from 'crypto';

export const v4 = (): string => randomUUID();
export default { v4 };
