let pendingReturnTo: string | undefined = undefined;

export function captureAuth0RedirectReturnTo(value: string | undefined): void {
  pendingReturnTo = value;
}

export function readPendingAuth0RedirectReturnTo(): string | undefined {
  return pendingReturnTo;
}
