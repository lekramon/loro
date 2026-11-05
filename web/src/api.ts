export type Profile = { acceptedFormats: string[]; acceptedContainers: string[]; acceptedResolutions: string[]; preferredResolution: string; audioPreferences: string[] };
export type Directories = { downloadDirectory: string; movieDirectory: string };
export type SettingsInput = { directories: Directories; profile: Profile };
export type Settings = SettingsInput & { revision: number; updatedAt: string };
export type SetupState = { completed: boolean; supportedResolutions: string[]; languages: { code: string; name: string }[] };
export type FieldError = { field: string; message: string };

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public fieldErrors: FieldError[] = []) { super(message); }
}

export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`/api/v1${path}`, options);
  const body = await response.json();
  if (!response.ok) throw new ApiError(response.status, body.code, body.message, body.fieldErrors);
  return body as T;
}
