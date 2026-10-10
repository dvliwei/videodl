import assert from 'node:assert/strict'
import { isIndeterminateProgress } from './format.js'

function test(name, fn) {
  try {
    fn()
    console.log(`ok - ${name}`)
  } catch (error) {
    console.error(`not ok - ${name}`)
    throw error
  }
}

test('unknown progress uses the indeterminate preparation state', () => {
  assert.equal(isIndeterminateProgress(null), true)
  assert.equal(isIndeterminateProgress(undefined), true)
  assert.equal(isIndeterminateProgress(Number.NaN), true)
  assert.equal(isIndeterminateProgress(Number.POSITIVE_INFINITY), true)
})

test('known progress uses the determinate progress state', () => {
  assert.equal(isIndeterminateProgress(0), false)
  assert.equal(isIndeterminateProgress(42), false)
  assert.equal(isIndeterminateProgress(100), false)
})
