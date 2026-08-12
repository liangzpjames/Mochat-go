import request from '@/utils/request'

export function login (params) {
  return request({
    url: '/user/auth',
    method: 'post',
    data: params
  })
}

export function logout (params) {
  return request({
    url: '/user/logout',
    method: 'put',
    data: params
  })
}
