<template>
  <div class="test-route">
    <h1>Test Route Page</h1>
    <p>If you can see this page, the routing system is working properly.</p>
    <el-button @click="testFunction">Test Button</el-button>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const message = ref('Hello World')

const testFunction = () => {
  console.log('Test function called')
  message.value = 'Button was clicked'
}
</script>

<style scoped>
.test-route {
  padding: 20px;
}
</style>