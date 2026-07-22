export function shouldBlockContactList(loading: boolean, itemCount: number): boolean {
  return loading && itemCount === 0
}
