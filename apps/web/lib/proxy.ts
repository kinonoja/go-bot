export const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function validDevEmail(value: string | undefined): value is string {
  return Boolean(value && value.length <= 100 && emailPattern.test(value));
}

export function proxyHeaders(contentType: string | null, devAuth: boolean, email: string | undefined): Headers {
  const headers = new Headers();
  if (contentType) headers.set("content-type", contentType);
  if (devAuth && validDevEmail(email)) headers.set("X-Dev-User-Email", email);
  return headers;
}
