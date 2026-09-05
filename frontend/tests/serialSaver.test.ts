import { expect, test } from 'bun:test'
import { createSerialSaver } from '../src/serialSaver'

test('old curl save cannot overwrite a newer msedge draft; writes are ordered', async () => {
  const writes: string[] = []
  let release!: (value: string) => void
  const save = createSerialSaver<{ name: string }>(async value => {
    writes.push(value.name)
    if (writes.length === 1) return new Promise(resolve => { release = resolve })
    return ''
  })
  const draft = { name: 'curl.exe' }
  const first = save(draft)
  await Promise.resolve()
  draft.name = 'msedge.exe'
  const second = save(draft)
  expect(writes).toEqual(['curl.exe'])
  release('')
  expect((await first).name).toBe('curl.exe')
  expect(draft.name).toBe('msedge.exe')
  expect((await second).name).toBe('msedge.exe')
  expect(writes).toEqual(['curl.exe', 'msedge.exe'])
})

test('failed save retains draft and does not block the next save', async () => {
  let attempt = 0
  const save = createSerialSaver<{ name: string }>(async () => ++attempt === 1 ? 'connected' : '')
  const draft = { name: 'msedge.exe' }
  await expect(save(draft)).rejects.toThrow('connected')
  expect(draft.name).toBe('msedge.exe')
  expect((await save(draft)).name).toBe('msedge.exe')
})
