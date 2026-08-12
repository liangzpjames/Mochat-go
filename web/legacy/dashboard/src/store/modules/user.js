import storage from 'store'
import { login } from '@/api/auth'

const user = {
  state: {
    token: '',
    roles: [],
    userInfo: null
  },

  mutations: {
    SET_TOKEN: (state, token) => {
      state.token = token
    },
    SET_ROLES: (state, roles) => {
      state.roles = roles
    },
    SET_USER_INFO: (state, userInfo) => {
      state.userInfo = userInfo
    }
  },

  actions: {
    // 登录
    Login ({ commit }, userInfo) {
      return new Promise((resolve, reject) => {
        login(userInfo).then(response => {
          const result = response.data
          const token = `Bearer ${result.token}`
          storage.set('ACCESS_TOKEN', token, result.expire * 60 * 1000)
          commit('SET_TOKEN', token)
          resolve()
        }).catch(error => {
          reject(error)
        })
      })
    },

    // 登出
    Logout ({ commit }) {
      return new Promise((resolve) => {
        commit('SET_TOKEN', '')
        storage.remove('ACCESS_TOKEN')
        resolve()
      })
    }

  }
}

export default user
