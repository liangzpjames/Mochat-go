import { z } from 'zod';

export type ApiEnvelope<T> = {
  code: number | string;
  msg: string;
  data: T;
};

export const apiEnvelopeSchema = z.object({
  code: z.union([z.number(), z.string()]),
  msg: z.string(),
  data: z.unknown(),
});

export function parseApiEnvelope<T>(input: unknown): ApiEnvelope<T> {
  return apiEnvelopeSchema.parse(input) as ApiEnvelope<T>;
}
