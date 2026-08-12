const getters = {
  isMobile: state => state.app.isMobile,
  token: state => state.user.token,
  roles: state => state.user.roles,
  userInfo: state => state.user.userInfo,
  addRouters: state => state.permission.addRouters,
  breadcrumb: state => state.permission.breadcrumb,
  permissionList: state => state.permission.permissionList,
  defaultRoutePath: state => state.permission.defaultRoutePath
}

export default getters
