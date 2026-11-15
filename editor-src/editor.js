import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { Markdown } from '@tiptap/markdown'
import Image from '@tiptap/extension-image'
import Placeholder from '@tiptap/extension-placeholder'
import { marked } from 'marked'

const root = document.querySelector('.article-writing')
if (root) {
  const source = root.querySelector('#article-body')
  const visual = root.querySelector('.article-editor-visual')
  const toolbar = root.querySelector('.article-toolbar')
  const status = root.querySelector('[data-editor-status]')
  const file = root.querySelector('[data-image-file]')
  const form = source.form
  let mode = 'visual', dirty = false, uploading = false
  function askFields(title, fields) {
    return new Promise(resolve => {
      const dialog=document.createElement('dialog')
      dialog.className='article-editor-dialog'
      const form=document.createElement('form'); form.method='dialog'
      const heading=document.createElement('h2'); heading.textContent=title; form.append(heading)
      const inputs=[]
      for (const field of fields) {
        const label=document.createElement('label'); label.textContent=field.label
        const input=document.createElement(field.multiline ? 'textarea' : 'input')
        input.value=field.value || ''; if (field.maxlength) input.maxLength=field.maxlength
        label.append(input); form.append(label); inputs.push(input)
      }
      const actions=document.createElement('div'); actions.className='article-dialog-actions'
      const cancel=document.createElement('button'); cancel.type='button'; cancel.textContent='Cancel'
      const save=document.createElement('button'); save.type='submit'; save.textContent='Save'
      cancel.addEventListener('click', () => dialog.close('cancel'))
      actions.append(cancel,save); form.append(actions); dialog.append(form); document.body.append(dialog)
      dialog.addEventListener('close', () => { const answer=dialog.returnValue==='cancel' ? null : inputs.map(i => i.value); dialog.remove(); resolve(answer) }, {once:true})
      dialog.showModal(); inputs[0]?.focus()
    })
  }
  const ImageNode = Image.extend({
    renderMarkdown: node => {
      const escape = value => String(value || '').replace(/\\/g, '\\\\').replace(/([\[\]\"])/g, '\\$1')
      const src = String(node.attrs?.src || '').replace(/ /g, '%20').replace(/\)/g, '%29')
      const title = node.attrs?.title || ''
      return `![${escape(node.attrs?.alt)}](${src}${title ? ` "${escape(title)}"` : ''})`
    },
    addNodeView() {
      return ({ node, getPos, editor }) => {
        let current = node
        const wrap = document.createElement('figure')
        wrap.className = 'article-image-block'
        const img = document.createElement('img')
        const captionEl = document.createElement('figcaption')
        const edit = document.createElement('button')
        const remove = document.createElement('button')
        edit.type = remove.type = 'button'
        edit.textContent = 'Edit image details'
        remove.textContent = 'Remove image'
        const update = n => { current = n; img.src = n.attrs.src; img.alt = n.attrs.alt || ''; captionEl.textContent = (n.attrs.title || '').startsWith('plink-caption:') ? n.attrs.title.slice(14) : ''; return true }
        update(node)
        edit.addEventListener('click', async () => {
          const prior=(current.attrs.title || '').startsWith('plink-caption:') ? current.attrs.title.slice(14) : ''
          const answer=await askFields('Image details', [
            {label:'Alt text', value:current.attrs.alt || '', maxlength:1000, multiline:true},
            {label:'Caption (optional)', value:prior, maxlength:1000},
          ])
          if (!answer) return
          const [alt,caption]=answer
          editor.chain().focus().setNodeSelection(getPos()).updateAttributes('image', {alt, title:caption ? 'plink-caption:' + caption : null}).run()
        })
        remove.addEventListener('click', () => editor.chain().focus().setNodeSelection(getPos()).deleteSelection().run())
        wrap.append(img, captionEl, edit, remove)
        return { dom: wrap, update, stopEvent: event => !!event.target.closest('button') }
      }
    },
  })
  const editor = new Editor({ element:visual, extensions:[StarterKit, Markdown, ImageNode, Placeholder.configure({placeholder:'Start writing…'})], content:source.value, contentType:'markdown', onUpdate:() => { dirty=true; updateButtons() }, onSelectionUpdate:() => updateButtons() })
  const canonical = value => {
    const doc = document.createElement('div')
    doc.innerHTML = marked.parse(value || '')
    return doc.innerHTML
  }
  function compatible(md) {
    try { return canonical(md) === canonical(editor.getMarkdown()) } catch { return false }
  }
  function sourceMode(message) {
    mode='source'; visual.hidden=true; source.hidden=false
    status.textContent=message || 'Markdown mode'
  }
  function visualMode() {
    editor.commands.setContent(source.value, {contentType:'markdown'})
    if (!compatible(source.value)) { sourceMode('This article contains Markdown the visual editor cannot safely preserve. Edit it in Markdown mode.'); return }
    dirty=false; mode='visual'; source.hidden=true; visual.hidden=false; status.textContent=''
  }
  function updateButtons() {
    for (const b of toolbar.querySelectorAll('[data-command]')) {
      const cmd=b.dataset.command
      b.classList.toggle('is-active', ['bold','italic','strike'].includes(cmd) && editor.isActive(cmd))
    }
  }
  visualMode(); toolbar.hidden=false
  toolbar.addEventListener('mousedown', e => { if (e.target.closest('button')) e.preventDefault() })
  toolbar.addEventListener('click', async e => {
    const button=e.target.closest('button[data-command]'); if (!button) return
    const cmd=button.dataset.command
    if (cmd === 'source') {
      if (mode === 'visual') { if (dirty) source.value=editor.getMarkdown(); sourceMode('Markdown mode'); }
      else visualMode()
      return
    }
    if (mode !== 'visual' || uploading) return
    const chain=editor.chain().focus()
    if (cmd==='bold') chain.toggleBold().run()
    if (cmd==='italic') chain.toggleItalic().run()
    if (cmd==='strike') chain.toggleStrike().run()
    if (cmd==='h2') chain.toggleHeading({level:2}).run()
    if (cmd==='quote') chain.toggleBlockquote().run()
    if (cmd==='bullet') chain.toggleBulletList().run()
    if (cmd==='ordered') chain.toggleOrderedList().run()
    if (cmd==='divider') chain.setHorizontalRule().run()
    if (cmd==='link') {
      const answer=await askFields('Add link', [{label:'URL',value:editor.getAttributes('link').href || 'https://'}])
      if (!answer) return
      const href=answer[0]
      if (!href) { editor.chain().focus().unsetLink().run(); return }
      try { const url=new URL(href); if (!['https:','http:','mailto:'].includes(url.protocol)) throw new Error(); editor.chain().focus().extendMarkRange('link').setLink({href}).run() }
      catch { status.textContent='Enter a valid http, https or mailto URL.' }
    }
    if (cmd==='image') file.click()
  })
  async function uploadImage(selected) {
    if (!selected || uploading) return
    if (!['image/png','image/jpeg','image/webp'].includes(selected.type)) { status.textContent='Choose a PNG, JPEG or WebP image.'; return }
    if (selected.size > 5*1024*1024) { status.textContent='Choose an image of 5 MB or less.'; file.value=''; return }
    uploading=true; editor.setEditable(false); status.textContent='Uploading image…'
    try {
      const data=new FormData(); data.append('image_file',selected)
      const token=document.cookie.match(/(?:^|;\s*)plink_csrf=([^;]*)/)
      const res=await fetch(root.dataset.uploadUrl,{method:'POST',body:data,headers:token?{'X-CSRF-Token':decodeURIComponent(token[1])}:{}})
      if (!res.ok) throw new Error((await res.text()).trim() || 'Upload failed')
      const result=await res.json()
      editor.setEditable(true)
      editor.chain().focus().setImage({src:result.url,alt:''}).run()
      status.textContent='Image added. Select Edit image details to add alt text or a caption.'
    } catch (e) { status.textContent=e.message || 'Upload failed. Try again.' }
    finally { uploading=false; editor.setEditable(true); file.value='' }
  }
  file.addEventListener('change', () => uploadImage(file.files[0]))
  visual.addEventListener('dragover', e => { if (e.dataTransfer?.types.includes('Files')) e.preventDefault() })
  visual.addEventListener('drop', e => {
    const item=e.dataTransfer?.files?.[0]
    if (!item) return
    e.preventDefault()
    const pos=editor.view.posAtCoords({left:e.clientX,top:e.clientY})
    if (pos) editor.commands.setTextSelection(pos.pos)
    uploadImage(item)
  })
  visual.addEventListener('paste', e => {
    const item=[...(e.clipboardData?.items || [])].find(i => i.kind==='file' && i.type.startsWith('image/'))
    if (item) { e.preventDefault(); uploadImage(item.getAsFile()) }
  })
  form.addEventListener('submit', e => {
    if (uploading) { e.preventDefault(); status.textContent='Wait for the upload to finish.'; return }
    if (mode==='visual' && dirty) source.value=editor.getMarkdown()
  })
}
