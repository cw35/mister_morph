import AppNavList from "./AppNavList";
import "./AppSidebar.css";

const AppSidebar = {
  components: {
    AppNavList,
  },
  props: {
    navItems: {
      type: Array,
      required: true,
    },
    currentPath: {
      type: String,
      required: true,
    },
  },
  emits: ["navigate"],
  template: `
    <aside class="sidebar">
      <AppNavList :navItems="navItems" :currentPath="currentPath" @navigate="$emit('navigate', $event)" />
    </aside>
  `,
};

export default AppSidebar;
