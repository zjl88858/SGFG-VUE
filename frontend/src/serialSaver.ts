// Serialize immutable snapshots. An old response must never reload/replace a
// form that the user has already edited again while the request was in flight.
export function createSerialSaver<T>(persist: (snapshot: T) => Promise<string>) {
  let tail: Promise<unknown> = Promise.resolve()
  return (value: T): Promise<T> => {
    const snapshot = structuredClone(value)
    const task = tail.then(async () => {
      const error = await persist(snapshot)
      if (error) throw new Error(error)
      return snapshot
    })
    tail = task.catch(() => undefined)
    return task
  }
}
