import AppNavList from "./AppNavList";

const AppMobileNavDrawer = {
  components: {
    AppNavList,
  },
  props: {
    modelValue: {
      type: Boolean,
      required: true,
    },
    title: {
      type: String,
      required: true,
    },
    navItems: {
      type: Array,
      required: true,
    },
    currentPath: {
      type: String,
      required: true,
    },
  },
  emits: ["update:modelValue", "close", "navigate"],
  template: `
    <QDrawer
      :modelValue="modelValue"
      @update:modelValue="$emit('update:modelValue', $event)"
      :title="title"
      placement="left"
      size="272px"
      :showMask="true"
      :maskClosable="true"
      :lockScroll="true"
      @close="$emit('close')"
    >
      <AppNavList
        :navItems="navItems"
        :currentPath="currentPath"
        :mobile="true"
        keyPrefix="drawer-"
        @navigate="$emit('navigate', $event)"
      />
    </QDrawer>
  `,
};

export default AppMobileNavDrawer;
