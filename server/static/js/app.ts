import { mount } from 'svelte';
import App from './SearchApp.svelte';

const appElement = document.getElementById('app');
if (appElement) {
  mount(App, { target: appElement });
} else {
  console.error('App mount target #app not found');
  mount(App, { target: document.body });
}
