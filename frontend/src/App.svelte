<script>
  import { onMount } from 'svelte';
  import { EventsOn } from '../wailsjs/runtime/runtime.js';
  let title='계약결혼한 남자의 비밀';
  let premise='계약결혼을 선택한 두 성인이 서로의 사정과 감춰 둔 이유를 알아간다.';
  let projects=[]; let selected=null; let error=''; let busy=false; let imported=[]; let recovered=[];
  let connection={state:'checking',message:'Windows에서 Abocado 연결을 확인하는 중입니다.'};
  const stageNames={script:'대본',images:'이미지',video:'영상',tts:'한국어 음성',lipsync:'립싱크',subtitles:'자막',edit:'편집'};
  const api=()=>window.go?.main?.App;
  async function refresh(){try{if(api()) projects=await api().ListProjects();}catch(e){error=String(e)}}
  async function checkConnection(){if(!api()){connection={state:'desktop_required',message:'Windows 데스크톱 앱에서 Abocado 연결을 확인할 수 있습니다.'};return}try{connection=await api().TestAbocadoConnection('')}catch(e){connection={state:'unavailable',message:String(e)}}}
  async function connectAbocado(){error='';busy=true;try{const message=await api().BeginAbocadoAuth('');connection={state:'authorizing',message};}catch(e){error=String(e)}finally{busy=false}}
  onMount(()=>{refresh();checkConnection();let off1=()=>{},off2=()=>{};if(api()){off1=EventsOn('abocado:connected',s=>{connection=s;error=''});off2=EventsOn('abocado:auth-failed',e=>{connection={state:'authentication_required',message:String(e)}})}return()=>{off1?.();off2?.()}});
  async function create(){error='';busy=true;try{if(api())selected=await api().CreateProject(title,premise);else selected=mockProject();if(!api())projects=[selected,...projects]}catch(e){error=String(e)}finally{busy=false}}
  function mockProject(){return {id:'preview-'+Date.now(),title,premise,target_seconds:300,resolution:'1920x1080',provider:'mock',stages:Object.fromEntries(Object.keys(stageNames).map(k=>[k,{status:'pending',attempts:0}]))}}
  async function runMock(){if(!selected)return;error='';busy=true;try{if(api())selected=await api().RunMock(selected.id);else{for(const s of Object.values(selected.stages))s.status='complete';}await refresh()}catch(e){error=String(e)}finally{busy=false}}
  async function importLocal(){error='';try{if(api())imported=await api().ChooseImportFolder();else error='데스크톱 앱에서만 폴더 선택과 로컬 복사를 사용할 수 있습니다.'}catch(e){error=String(e)}}
  async function recoverExisting(){error='';busy=true;try{if(!api())throw new Error('데스크톱 에이전트가 필요합니다.');recovered=await api().RecoverExistingAssets('')}catch(e){error=String(e)}finally{busy=false}}
</script>

<svelte:head><title>K-Drama 5분 자동 제작</title><meta name="description" content="한국 드라마 제작 파이프라인"/></svelte:head>
<main>
  <header><div class="brand">FRAME<span>SEOUL</span></div><div class="badge"><i></i> 로컬 작업공간</div></header>
  <section class="hero"><p class="eyebrow">KOREAN DRAMA WORKFLOW · 05:00</p><h1>이야기를, 장면으로.<br/><em>한 편의 드라마로.</em></h1><p class="sub">주제에서 300초 완성본까지. 각 단계를 저장하고 언제든 이어서 만들 수 있습니다.</p></section>
  <section class="layout">
    <article class="panel setup"><div class="panel-head"><span class="step">01</span><h2>새 프로젝트</h2></div>
      <label>프로젝트 제목<input bind:value={title}/></label><label>이야기 주제<textarea rows="4" bind:value={premise}></textarea></label>
      <div class="specs"><div><small>목표 길이</small><strong>05:00</strong></div><div><small>출력</small><strong>1080p · 24fps</strong></div><div><small>영상 구조</small><strong>25 × 12초</strong></div></div>
      <button class="primary" onclick={create} disabled={busy}>{busy?'저장 중…':'프로젝트 만들기'} <span>↗</span></button>
    </article>
    <article class="panel flow"><div class="panel-head"><span class="step">02</span><h2>제작 파이프라인</h2><span class="free">무료 모의 실행</span></div>
      <div class="stages">{#each Object.entries(stageNames) as [key,name],i}<div class="stage" class:done={selected?.stages?.[key]?.status==='complete'}><div class="stage-icon">{selected?.stages?.[key]?.status==='complete'?'✓':String(i+1).padStart(2,'0')}</div><div><b>{name}</b><small>{selected?.stages?.[key]?.status==='complete'?'완료':selected?.stages?.[key]?.status==='failed'?'오류 · 재개 가능':'대기 중'}</small></div><span class="arrow">{i<Object.keys(stageNames).length-1?'›':''}</span></div>{/each}</div>
      <button class="secondary" onclick={runMock} disabled={busy||!selected}>{busy?'실행 중…':'무료 모의 파이프라인 실행'} <span>▶</span></button>
      <button class="quiet" onclick={importLocal}>로컬 폴더에서 기존 자산 가져오기 <span>↗</span></button>
      <button class="quiet" onclick={recoverExisting} disabled={busy||connection.state!=='connected'}>완료된 Abocado 작업에서 기존 결과 복구·다운로드 <span>↓</span></button>
    </article>
  </section>
  <section class="bottom"><div><span class="dot"></span><b>크레딧 보호 켜짐</b><p>기존 자산 복구와 연결 테스트는 생성 비용을 쓰지 않습니다.</p></div><div><span class="dot" class:muted={connection.state!=='connected'}></span><b>Abocado · {connection.state==='connected'?'연결됨':connection.state==='authorizing'?'인증 대기':connection.state==='checking'?'확인 중':'연결 필요'}</b><p>{connection.message}</p></div><div><span class="dot muted"></span><b>FFmpeg 로컬 렌더</b><p>복구된 기존 미디어를 Windows PC에서 편집합니다.</p></div></section>
  <section class="connection-card"><div><b>Abocado 연결 상태</b><p>{connection.message}</p><small>최초 1회는 브라우저에서 승인합니다. 인증값은 이 Windows 사용자 계정에 암호화해 저장됩니다.</small></div><div class="connection-actions"><button class="quiet" onclick={checkConnection} disabled={busy}>연결 테스트</button>{#if connection.state==='authentication_required'}<button class="secondary" onclick={connectAbocado} disabled={busy}>브라우저에서 최초 인증</button>{/if}</div></section>
  {#if selected}<aside class="notice">프로젝트 <strong>{selected.title}</strong> 저장됨 · 작업 ID {selected.id}</aside>{/if}
  {#if imported.length}<aside class="notice">로컬 자산 {imported.length}개를 찾았습니다.</aside>{/if}
  {#if recovered.length}<section class="recovered"><h3>기존 생성 결과 · 재생성 없이 복구</h3><p>작업 기록의 12초 영상 1개와 TTS 4개를 내려받았습니다. 기존 대사와 현재 대본의 인물/내용이 일치하는지 편집 전에 확인하세요.</p>{#each recovered as item}<div class="asset-row"><span class:ok={item.status==='downloaded'} class="asset-dot"></span><b>{item.kind==='video'?'12초 영상':'TTS 음성'}</b><code>{item.job_key}</code><small>{item.status==='downloaded'?item.local_path:item.error||item.status}</small></div>{/each}</section>{/if}
  {#if error}<aside class="error">{error}</aside>{/if}
  <footer>STORY FIRST · MADE IN KOREA <span>© K-DRAMA STUDIO</span></footer>
</main>

