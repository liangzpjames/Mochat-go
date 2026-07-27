<template>
  <div class="loss-customers">
    <a-card>
      <div class="filter-form">
        <!--            所属客服-->
        <div class="item">
          <label>归属成员：</label>
          <div class="input belongTo" @click="selectMemberShow">
            <span class="tips" v-if="employees.length==0">请选择成员</span>
            <a-tag v-for="(item,index) in employees" :key="index">{{ item.name }}</a-tag>
          </div>
        </div>
        <div class="item">
          <a-button @click="resetBtn">重置</a-button>
        </div>
      </div>
      <div class="table-wrapper">
        <a-table
          :columns="columns"
          :data-source="tableData"
          :rowKey="record => record.id"
          :pagination="pagination"
          @change="handleTableChange">
          <div slot="detail" slot-scope="text, record">
            <div class="detail-box">
              <img :src="record.avatar"/>
              <span>{{ record.name }}</span>
              <span style="color: #3CD389">@微信</span>
            </div>
          </div>
          <div slot="tag" slot-scope="text, record">
            <span v-if="record.tag.length !== 0">{{ tagJoin(record.tag) }}</span>
            <span v-else>--</span>
          </div>
        </a-table>
      </div>
    </a-card>
    <selectMember ref="selectMember" @change="peopleChange"></selectMember>
  </div>
</template>

<script>
import { getLossContactList } from '@/api/lossContact'
import selectMember from '@/components/Select/member'
export default {
  components: {
    selectMember
  },
  data () {
    return {
      columns: [
        {
          title: '客户信息',
          dataIndex: 'detail',
          align: 'center',
          scopedSlots: { customRender: 'detail' }
        },
        {
          title: '标签',
          dataIndex: 'tag',
          align: 'center',
          scopedSlots: { customRender: 'tag' }
        },
        {
          title: '归属成员',
          dataIndex: 'employeeName',
          align: 'center',
          scopedSlots: { customRender: 'employeeName' }

        },
        {
          title: '备注',
          dataIndex: 'remark',
          align: 'center'
        },
        {
          title: '删除时间',
          dataIndex: 'deletedAt',
          align: 'center'
        }
      ],
      tableData: [],
      employeeIdList: '',
      pagination: {
        total: 0,
        current: 1,
        pageSize: 10,
        showSizeChanger: true
      },
      employees: []
    }
  },
  created () {
    this.getTableData()
  },
  methods: {
    confirmChange () {
      this.getTableData()
    },
    selectMemberShow () {
      this.$refs.selectMember.setSelect(this.employees)
    },
    getTableData () {
      const params = {
        employeeId: this.employeeIdList,
        page: this.pagination.current,
        perPage: this.pagination.pageSize
      }
      getLossContactList(params).then(res => {
        this.tableData = res.data.list
        this.pagination.total = res.data.page.total
      })
    },
    handleTableChange ({ current, pageSize }) {
      this.pagination.current = current
      this.pagination.pageSize = pageSize
      this.getTableData()
    },
    // 成员选择
    peopleChange (e) {
      this.employees = e
      const arr = []
      this.employees.map(i => {
        arr.push(i.employeeId)
      })
      this.employeeIdList = arr.join(',')
    },
    tagJoin (value) {
      if (Array.isArray(value)) {
        return value.join(',')
      }
      return Object.values(value).join(',')
    },
    resetBtn () {
      this.employees = []
      this.employeeIdList = ''
      this.pagination.current = 1
      this.getTableData()
    }
  }
}
</script>

<style lang="less" scoped>
.filter-form {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  .item {
    min-width: 266px;
    margin-top: 16px;
    padding-right: 33px;
    box-sizing: content-box;
    display: flex;
    justify-content: flex-start;
    align-items: center;

    .input {
      width: 208px;
      padding-left: 12px;
    }
    .belongTo{
      padding-left: 7px;
      border: 1px solid #D9D9D9;
      height: 32px;
      line-height: 32px;
      cursor: pointer;
      .tips{
        color: #BFBFBF;
      }
    }
    .ant-select {
      width: 100%;
    }
  }
}
.loss-customers {
  .table-wrapper {
    margin-top: 20px;
    .detail-box {
      display: flex;
      align-items: center;
      img {
        width: 40px;
        height: 40px;
      }
      span {
        margin-left: 20px;
      }
    }
  }
}
</style>
