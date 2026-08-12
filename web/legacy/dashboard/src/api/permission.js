import request from '@/utils/request'

export function permissionByUser (params) {
  return request({
    url: '/role/permissionByUser',
    method: 'get',
    data: params
  })
}
