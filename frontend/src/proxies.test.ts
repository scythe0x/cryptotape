import { expect, it } from 'vitest'
import { nextProxyName, parseProxy } from './proxies'

it('parses DNS and local SOCKS addresses without changing existing credentials', () => {
  expect(parseProxy('route.example : 443')).toEqual({
    host: 'route.example',
    port: 443,
  })
  expect(parseProxy('127.0.0.1:1080')).toEqual({
    host: '127.0.0.1',
    port: 1080,
  })
  expect(parseProxy('https://route.example')).toMatchObject({
    host: 'route.example',
    port: 443,
  })
  expect(parseProxy('socks5://[::1]:1081')).toMatchObject({
    host: '::1',
    port: 1081,
  })
})

it('parses pasted credentials locally without losing special characters', () => {
  expect(parseProxy('route.example : 443\nЛогин: test-user\nПароль: test:@password')).toEqual({
    host: 'route.example',
    port: 443,
    login: 'test-user',
    password: 'test:@password',
  })
  expect(parseProxy('http://user%40test:pass%3Aword@route.example:8080')).toMatchObject({
    login: 'user@test',
    password: 'pass:word',
  })
})

it.each(['', 'route.example:99999', 'route.example:0', 'https://route.example/path', 'ss://route.example:1080'])(
  'rejects invalid proxy address %s',
  (address) => {
    expect(parseProxy(address)).toBeNull()
  },
)

it('suggests the first unused proxy name', () => {
  expect(nextProxyName([])).toBe('proxy-1')
  const profiles = [
    {
      id: 'one',
      name: 'proxy-1',
      host: 'localhost',
      port: 1080,
      protocol: 'socks5' as const,
      login: '',
      password: '',
    },
  ]
  expect(nextProxyName(profiles)).toBe('proxy-2')
})
