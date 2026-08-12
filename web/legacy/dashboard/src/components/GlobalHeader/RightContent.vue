<template>
  <div :class="wrpCls">
    <avatar-dropdown :menu="showMenu" :class="prefixCls" />
  </div>
</template>

<script>
import AvatarDropdown from './AvatarDropdown'

export default {
  name: 'RightContent',
  components: {
    AvatarDropdown
  },
  props: {
    prefixCls: {
      type: String,
      default: 'ant-pro-global-header-index-action'
    },
    isMobile: {
      type: Boolean,
      default: () => false
    },
    theme: {
      type: String,
      required: true
    }
  },
  data () {
    return {
      showMenu: true
    }
  },
  computed: {
    wrpCls () {
      return {
        'ant-pro-global-header-index-right': true,
        'ant-pro-global-header-index-light': true
      }
    }
  }
}
</script>
