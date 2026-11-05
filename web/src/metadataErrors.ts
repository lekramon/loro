import { ApiError } from './api';
import type { MessageKey } from './messages';
export function metadataError(error: unknown, t: (key: MessageKey) => string): string {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'metadata-unconfigured': return t('No TMDB token is configured. Save a Read Access Token in settings.');
      case 'metadata-authentication-failed': return t('TMDB rejected the token. Use the API Read Access Token, not the API key, and save it again.');
      case 'metadata-timeout': return t('TMDB timed out. Check the service internet connection and try again.');
      case 'metadata-rate-limited': return t('TMDB is rate limiting requests. Wait before trying again.');
      case 'metadata-invalid-response': return t('TMDB returned an invalid response. Try again later.');
      case 'validation-failed': return t('Enter a movie title and a year between 1870 and 2100, or leave the year blank.');
    }
  }
  return t('Unable to reach movie metadata. Check the service internet connection and try again.');
}
