export const products = [
 {id:'kedinasan',kind:'tryout',title:'Tryout Kedinasan 03',program:'Kedinasan',description:'Latihan TWK, TIU, dan TKP dengan evaluasi per subtes.',price:50000,icon:'shield',tone:'warm',duration:'90 menit',questions:100,access:'Akses sampai 30 November 2026'},
 {id:'polri',kind:'tryout',title:'Tryout POLRI 02',program:'POLRI',description:'Latih pengetahuan, penalaran, dan sikap kerja secara terarah.',price:45000,icon:'book',tone:'',duration:'90 menit',questions:100,access:'Akses sampai 30 November 2026'},
 {id:'campur',kind:'kecermatan',title:'Kecermatan Campur',program:'Kecermatan',description:'Temukan item yang hilang dari kombinasi angka dan simbol.',price:25000,icon:'spark',tone:'green',duration:'10 kolom',questions:50,credits:5,access:'5 kesempatan tes'},
 {id:'simbol',kind:'kecermatan',title:'Kecermatan Simbol',program:'Kecermatan',description:'Bangun ritme dan ketelitian lewat rangkaian simbol.',price:20000,icon:'spark',tone:'green',duration:'10 kolom',questions:50,credits:5,access:'5 kesempatan tes'}
];
export const subtests = [
 {name:'Tes Wawasan Kebangsaan',short:'TWK',questions:[
  {prompt:'Sikap yang paling mencerminkan musyawarah dalam sebuah tim adalah …',type:'pg',options:['Memaksakan keputusan ketua','Mendengarkan pendapat sebelum mengambil keputusan','Memilih pendapat yang paling cepat','Menghindari diskusi agar tidak berbeda pendapat'],key:1,explanation:'Musyawarah memberi ruang bagi semua anggota untuk menyampaikan pendapat sebelum keputusan diambil.'},
  {prompt:'Menghargai perbedaan pendapat membantu menjaga persatuan dalam kelompok.',type:'boolean',options:['Benar','Salah'],key:0,explanation:'Sikap saling menghargai memungkinkan kelompok tetap bekerja sama meskipun pendapatnya berbeda.'}
 ]},
 {name:'Tes Intelegensia Umum',short:'TIU',questions:[
  {prompt:'Perhatikan deret berikut: 3, 6, 12, 24, … Angka berikutnya adalah …',type:'pg',options:['30','36','42','48'],key:3,explanation:'Setiap angka merupakan dua kali angka sebelumnya. Jadi, 24 × 2 = 48.'},
  {prompt:'Sebuah tim menyelesaikan 18 tugas dalam 3 jam dengan laju tetap. Berapa tugas yang selesai dalam 5 jam?',type:'text',key:'30',explanation:'Laju pengerjaan adalah 18 ÷ 3 = 6 tugas per jam. Dalam 5 jam: 6 × 5 = 30 tugas.'}
 ]},
 {name:'Tes Karakteristik Pribadi',short:'TKP',questions:[
  {prompt:'Pilih semua tindakan yang mendukung kerja sama ketika tim menghadapi tenggat waktu.',type:'multi',options:['Membagi tugas sesuai kemampuan anggota','Menyampaikan hambatan lebih awal','Menyembunyikan keterlambatan','Membantu anggota yang membutuhkan'],key:[0,1,3],explanation:'Pembagian tugas, komunikasi hambatan, dan saling membantu mendukung kerja sama. Menyembunyikan hambatan justru menambah risiko.'},
  {prompt:'Ketika mendapat masukan atas pekerjaanmu, tindakan yang paling konstruktif adalah …',type:'pg',options:['Menolak karena pekerjaan sudah selesai','Mendengarkan, meminta contoh, lalu memperbaiki','Membiarkan orang lain mengerjakan ulang','Menghindari pembicaraan'],key:1,explanation:'Mendengarkan dan memahami masukan membuat perbaikan lebih tepat sasaran.'}
 ]}
];
export const scoreAnswer=(q,a)=>q.type==='multi'?JSON.stringify([...(a||[])].sort())===JSON.stringify(q.key):q.type==='text'?String(a??'').trim()===q.key:a===q.key;
export const baseReferences = ['▲ ● ◆','■ ◆ ●','● ▲ ■','◆ ■ ▲','▲ ◆ ■'];
export function references(id,column){const n=(column%5);return baseReferences.map((v,i)=>id==='simbol'?baseReferences[(i+n)%5]:`${(i+n+2)%10} ${['▲','◆','●','■','✦'][(i+n)%5]} ${(i*3+n+1)%10}`);}
export const missingFor=(column,question)=>(question*3+(column%5)*2)%5;
export function displayLabels(column,question){const missing=missingFor(column,question);const all=[0,1,2,3,4].filter(i=>i!==missing);const shift=question%4;return [...all.slice(shift),...all.slice(0,shift)];}
