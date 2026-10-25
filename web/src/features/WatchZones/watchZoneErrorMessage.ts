import { ApiRequestError } from '../../api/client';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

const MESSAGES_BY_CODE: Readonly<Record<string, string>> = {
  zone_name_taken: 'You already have a watch zone with that name. Pick a different one.',
  boundary_requires_paid_tier:
    'Drawing your own shape needs a paid plan. Use a circle instead, or upgrade.',
  boundary_invalid: 'That shape is not valid. Redraw it so its edges do not cross.',
  boundary_too_large: 'That shape is too big for your plan. Draw a smaller area.',
  filter_requires_pro_tier: 'Filters are part of the Pro plan. Remove the filter, or upgrade.',
  filter_key_invalid: 'We did not recognise that filter. Pick one from the list.',
};

export function watchZoneErrorMessage(err: unknown): string {
  if (err instanceof ApiRequestError && err.code !== undefined) {
    const mapped = MESSAGES_BY_CODE[err.code];
    if (mapped) return mapped;
  }
  return extractErrorMessage(err);
}
