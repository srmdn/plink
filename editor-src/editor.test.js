import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { JSDOM } from 'jsdom'

const bundle = await readFile(new URL('../web/js/article-editor.js', import.meta.url), 'utf8')
async function setup() {
  const html = `<form><div class="article-writing" data-upload-url="/admin/articles/media"><div class="article-toolbar" hidden><button type="button" data-command="image">Insert image</button><input type="file" data-image-file hidden></div><div class="article-editor-visual" hidden></div><textarea id="article-body" name="body_md"></textarea><p data-editor-status></p></div></form>`
  const dom = new JSDOM(html, { url:'http://localhost', runScripts:'outside-only', pretendToBeVisual:true })
  let uploads=0
  dom.window.fetch=async () => { uploads++; return {ok:true, json:async()=>({url:'/media/test.webp'})} }
  dom.window.eval(bundle)
  const editor=dom.window.document.querySelector('.tiptap')
  assert.ok(editor,'visual editor initialized')
  return {dom, editor, uploads:()=>uploads}
}
async function waitForImage(env) {
  for (let i=0; i<50 && !env.editor.querySelector('img'); i++) await new Promise(resolve=>setTimeout(resolve,10))
  assert.equal(env.uploads(),1)
  assert.equal(env.editor.querySelector('img')?.getAttribute('src'),'/media/test.webp')
  env.dom.window.close()
}
test('paste uploads an image and inserts a block', async () => {
  const env=await setup(); const file=new env.dom.window.File(['data'],'image.png',{type:'image/png'})
  const event=new env.dom.window.Event('paste',{bubbles:true,cancelable:true})
  Object.defineProperty(event,'clipboardData',{value:{items:[{kind:'file',type:'image/png',getAsFile:()=>file}],files:[file],getData:()=>'',types:['Files']}})
  env.editor.dispatchEvent(event)
  await waitForImage(env)
})
test('drop uploads an image and inserts a block', async () => {
  const env=await setup(); const file=new env.dom.window.File(['data'],'image.png',{type:'image/png'})
  env.dom.window.document.elementFromPoint=()=>env.editor
  const event=new env.dom.window.Event('drop',{bubbles:true,cancelable:true})
  Object.defineProperty(event,'dataTransfer',{value:{files:[file],types:['Files'],getData:()=>'',clearData:()=>{}}})
  Object.defineProperty(event,'clientX',{value:0}); Object.defineProperty(event,'clientY',{value:0})
  env.editor.dispatchEvent(event)
  await waitForImage(env)
})
