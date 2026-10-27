ÖTÜKEN-MATRİS: Yeni Nesil Donanımsız Savunma Yazılımı Mimari ve Geliştirme 
Dokümantasyonu
Modern harp sahasında asimetrik tehditler ve dijitalleşen savaş konseptleri, fiziksel 
donanımların ötesinde, veriyi işleyen ve karar mekanizmalarını yöneten yazılım 
mimarilerinin stratejik üstünlüğünü tartışmasız bir şekilde ortaya koymuştur. Küresel 
çapta savunma stratejileri, izole edilmiş ve silolaşmış donanım sistemlerinden, ağ 
merkezli ve tamamen entegre "Müşterek Tüm Alanlarda Komuta ve Kontrol" (CJADC2) 
konseptlerine doğru hızla evrilmektedir.1 Bu rapor, donanımdan bağımsız 
(hardware-agnostic), tamamen algoritmik ve yazılım tabanlı bir C4ISR (Komuta, 
Kontrol, Muhabere, Bilgisayar, İstihbarat, Gözetleme ve Keşif) platformu olan 
ÖTÜKEN-MATRİS sisteminin detaylı mühendislik ve yazılım mimarisi dokümantasyonunu 
sunmaktadır. Geleneksel isimlendirmelerden sıyrılarak Türkçe kökenli ve modern bir 
tınıyla adlandırılan bu sistem, "Görünmeyeni gör, duyulmayanı duy, emredileni yap" 
mottosuyla tasarlanmış, 5 ana küme (GÖRGÜ, KIN, KOVAN, BUYRUK, ULAK) altında toplanan 
toplam 60 modülden oluşan devasa bir dijital beyindir.

ÖTÜKEN-MATRİS, kendisi herhangi bir sensör, kamera veya radar üretmeyen, ancak 
sahadaki tüm sensörlerden (optik, termal, akustik, radar, yazılım tanımlı radyolar) 
gelen ham veriyi standart API'ler ve veri yolları üzerinden alıp işleyen merkezi bir 
platformdur. Sistemin en temel güvenlik felsefesi, "Sıfır Güven" (Zero Trust) 
mimarisine dayanmaktadır. Savunma Bakanlığı (DoD) standartlarına uygun olarak inşa 
edilecek bu mimari, "asla güvenme, daima doğrula" prensibiyle çalışarak sürekli 
kimlik doğrulama, mikro-segmentasyon ve en az yetki (least privilege) ilkelerini 
sistemin her bir fonksiyonuna entegre edecektir.3 ZTA (Zero Trust Architecture) 
yaklaşımı, sistem altyapısının siber tehditlere karşı yapısal bir bağışıklık 
kazanmasını sağlarken, donanım bağımsız yazılım yaklaşımı da eski ve yeni tüm savunma 
envanterinin tek bir matris üzerinden yönetilebilmesine olanak tanıyacaktır.5 Bu 
dokümantasyon, ÖTÜKEN-MATRİS'in geliştirilme hiyerarşisini, modüllerin yazılımsal 
altyapılarını, kullanılacak matematiksel modelleri ve entegrasyon süreçlerini en ince 
ayrıntısına kadar incelemektedir.

Geliştirme Sıralaması ve Fazlandırma Stratejisi
ÖTÜKEN-MATRİS gibi hiper-kompleks, 60 modüllü bir yazılım ekosisteminin inşası, 
rastgele veya eşzamanlı bir modül geliştirmesiyle başarılamaz. Donanımsız bir yazılım 
platformunda veri akışı olmadan hiçbir yapay zeka modeli eğitilemez, hiçbir görüntü 
işleme algoritması test edilemez ve hiçbir otonom sürüş mantığı kurgulanamaz. Bu 
sebeple, sistemin omurgası olan iletişim ve veritabanı altyapısı ilk sıraya 
yerleştirilmelidir. Geliştirme süreci, biyolojik bir organizmanın evrimsel inşasını 
taklit eden kesin bir mantıksal sıraya oturtulmuştur.
Aşağıdaki tabloda, modüllerin hangi mantıksal silsile ile kodlanması gerektiği ve bu 
sıralamanın mimari gerekçeleri özetlenmektedir:
Faz Sırası
Küme Adı
Biyolojik Karşılığı
Geliştirme Gerekçesi ve Entegrasyon Rolü
Faz 1
ULAK
Sinir Sistemi ve Omurga
Modüller arası iletişimi, asenkron veri akışını, veritabanını ve altyapı ağını 
kurmadan diğer modüllerin çalıştırılması teknik olarak imkansızdır. Mikroservis 
orkestrasyonu ilk adımdır.
Faz 2
GÖRGÜ
Duyu Organları
Altyapı kurulduktan sonra, dış dünyadan veri (görüntü, ses, sinyal, OSINT) toplayan 
sensör işleme algoritmalarının sisteme beslenmesi gerekir. Yapay zekanın hammaddesi 
bu aşamada üretilir.
Faz 3
BUYRUK
Beyin ve Biliş
Toplanan ham verinin yapay zeka ile anlamlandırılması, stratejik kararların alınması, 
etik çerçevelerin belirlenmesi ve kaynakların önceliklendirilmesi aşamasıdır. Karar 
motorları burada inşa edilir.
Faz 4
KOVAN
Kas ve Uzuvlar
Alınan stratejik kararların sahadaki fiziksel platformlara (dron sürüleri, insansız 
yer araçları) iletilmesi, otonom navigasyon ve swarm (sürü) algoritmalarının hayata 
geçirilmesidir.
Faz 5
KIN
Bağışıklık Sistemi
Sistem tamamen operasyonel ve otonom hale geldikten sonra, dış siber saldırılara, 
elektronik harbe (jamming/spoofing) ve kuantum tehditlerine karşı korunması için 
kalkan katmanı enjekte edilir.

Bu stratejik fazlandırma sayesinde, yazılım ekipleri blokaj yaşamadan (dependency 
hell) birbirini izleyen sprintler halinde ilerleyebilecektir. Aşağıdaki bölümlerde, 
bu geliştirme sırasına harfiyen sadık kalınarak, her bir küme ve altındaki 12 modülün 
yazılım mühendisliği gereksinimleri ayrıntılı olarak açıklanmaktadır.

Faz 1: ULAK Kümesi (Altyapı ve Veri İletişimi)
ULAK kümesi, ÖTÜKEN-MATRİS sisteminin sarsılmaz dijital sinir ağıdır. Yazılım 
projesinin temel atma töreni niteliğindeki bu küme, üzerinde tüm diğer algoritmaların 
koşacağı dağıtık mimariyi, durum yönetimini (state management) ve veri iletişimini 
koordine eder. Bu küme tamamlanmadan sahadan veri almak veya komut göndermek mümkün 
değildir.

BÜTÜN: Ana Çatı Sistemi
Tüm modülleri birleştiren mikroservis mimarisinin ana kapsayıcısı olan BÜTÜN modülü, 
sistemin kalbidir. Bu yapı, Kubernetes veya benzeri gelişmiş konteyner tabanlı 
orkestrasyon araçları kullanılarak tasarlanmalı ve sistemin yatayda (horizontal) ve 
dikeyde (vertical) esnek bir şekilde ölçeklenebilmesini sağlamalıdır.7 BÜTÜN, "Kral" 
sistem olarak, tüm alt kümelerin birbirleriyle olan bağımlılıklarını yöneten ana bir 
servis ağı (Service Mesh) ve API ağ geçidi (API Gateway) işlevini görür. Yazılımsal 
olarak tüm süreçlerin izolasyonunu sağlarken, kaynakların adil dağıtımını da işletim 
sistemi seviyesinde denetler.

BAĞ: Veri Transfer Katmanı
Sahadaki binlerce sensörden ve işlemciden gelen verilerin anlık olarak doğru 
modüllere iletilmesi için BAĞ modülü geliştirilir. Bu modül, modüller arası asenkron 
iletişim altyapısını temsil eder. Düşük gecikmeli (low-latency) veri aktarımı için 
gRPC iletişim protokolü ve Apache Kafka benzeri yüksek ölçeklenebilir olay güdümlü 
(event-driven) mesajlaşma kuyrukları kullanılır.9 Sistemdeki anlık kesintilere karşı 
veri kaybını önlemek amacıyla yüksek toleranslı mesaj doğrulama ve tekrar iletim 
mekanizmalarını bünyesinde barındırır.
KÜTÜK: Anlık Durum Veritabanı
Savaş sahasındaki dinamik objelerin gerçek zamanlı durumlarının (Real-time State) 
saklandığı dağıtık bellek içi (in-memory) veritabanı çözümüdür. Dağıtık sistemlerde 
lider seçimi, tutarlılık (consistency) ve log replikasyonu gibi kritik görevler için 
RAFT veya Paxos konsensüs algoritmaları kullanılarak, ağ kopmaları veya donanım 
arızaları durumunda dahi veritabanı bütünlüğü garanti altına alınır.10 KÜTÜK, SSD ve 
DRAM entegrasyonuna dayalı hibrit bellek mimarisi kullanarak veri okuma ve yazma 
hızlarını milisaniyenin altına, mikrosaniye seviyelerine indirmeyi amaçlar.13
DİRLİK: Sistemin Genel Sağlık Taraması
Bu modül, sistemin sürekli nabzını tutan bir telemetri ve izleme yazılımıdır. Tüm 
servislerin işlemci (CPU), bellek (RAM), disk I/O ve ağ gecikme değerlerini 
milisaniye bazında ölçerek anormallikleri saptar. Mikroservis mimarilerinde sıkça 
karşılaşılan servis çökmeleri durumunda, DİRLİK modülü anında BÜTÜN modülüne bir geri 
bildirim döngüsü göndererek, çöken konteynerin yerine derhal yedek bir konteynerin 
(replica) ayağa kalkmasını tetikler.
BAĞDAŞ: Farklı Dillerdeki Kodları Konuşturma
Modern savunma yazılımlarında tek bir programlama dili kullanımı, performans ve 
esneklik açısından ciddi dar boğazlar yaratır. BAĞDAŞ, farklı dillerdeki bileşenleri 
(polyglot architecture) kayıpsız bir şekilde birbirine bağlayan Foreign Function 
Interface (FFI) modülüdür. Çekirdek şifreleme ve mutlak bellek güvenliği gerektiren 
düşük seviyeli kritik modüller, C seviyesi hız sunarken aynı zamanda derleme 
zamanında bellek yönetimini doğrulayan Rust dili ile yazılır.14 Öte yandan, yapay 
zeka modelleri Python ile kodlanırken, eski nesil NATO donanımları C++ ile sisteme 
dahil edilir; BAĞDAŞ bu dil bariyerlerini köprüler.
AKIM: Veri Trafiğini Yoğunluğa Göre Yönlendirme
Savaş anında ağ üzerinde devasa boyutlarda veri paketleri dolaşır. AKIM modülü, 
sahadan gelen yüksek çözünürlüklü video akışlarının, sensör telemetrilerinin ve 
şifreli ses iletişimlerinin bant genişliğini dinamik olarak optimize eder. Yapay zeka 
destekli bir yük dengeleyici (load balancer) mantığıyla çalışan bu kod bloğu, ağ 
üzerindeki tıkanıklıkları anlık olarak tespit ederek veri trafiğini en uygun uç 
noktalara (edge nodes) veya daha müsait röle sunucularına yönlendirir.
YURT: Uç Bilişim ve Yerel Depolama
İletişimin veya GPS bağlantısının koptuğu (denied environments) zorlu ortamlarda 
sistemin çalışmaya devam etmesini sağlayan uç bilişim (Edge Computing) mimarisidir. 
YURT, verileri sadece bulutta değil, yerel cihazlarda (edge servers) tutarak asenkron 
senkronizasyon algoritmaları çalıştırır.17 İletişim tekrar sağlandığında, Sidecar 
mimarisiyle çalışan YurtHub benzeri proxy mekanizmaları aracılığıyla yerel veriler 
ana bulut ile çift yönlü olarak ve herhangi bir veri uyuşmazlığına (conflict) mahal 
vermeden eşitlenir.7
YÜZEY: 3D Kullanıcı Arayüzü (UI)
Komutanların sahadaki kaosu net bir şekilde okuyabilmesi için tasarlanan taktiksel 
izleme ekranıdır. Geleneksel iki boyutlu (2D) statik haritalar yerine, Unreal Engine 
veya Cesium gibi gelişmiş 3D oyun ve görselleştirme motorlarının mimarileri temel 
alınarak inşa edilir.19 Lidar, fotogrametri ve uydu verilerini kullanarak sahayı 
voksel (voxel) ve çokgen (polygon) bazlı fotogerçekçi bir "Dijital İkiz" (Digital 
Twin) olarak render eden YÜZEY, komutana tam bir durumsal farkındalık sağlar.21
İM: Dijital İmza ve Kimlik
Sıfır Güven (Zero Trust) mimarisinin kalbinde yer alan kimlik doğrulama modülüdür. 
Sahadaki her bir birimin, piyadenin veya donanımın MATRİS ağına katılırken kendini 
doğrulaması için gelişmiş asimetrik kriptografi kullanır. Yetkisiz girişleri veya 
çalınan cihazların sisteme sızmasını önlemek için her birime özel, sürekli değişen 
(rotating) token tabanlı şifrelemeler ve açık anahtar altyapısı (PKI) uygulanarak ağ 
giriş çıkışları katı bir şekilde denetlenir.
YAYIN: Şifreli ve Sıkıştırılmış Veri Yayımı
Komuta merkezinden çıkan hassas emirlerin, koordinatların veya yazılım 
güncellemelerinin sahadaki platformlara aktarılması sırasındaki veri yayım 
protokolüdür. İletişim bant genişliğinin çok dar olduğu cephe koşullarında, YAYIN 
modülü verileri gelişmiş kayıpsız sıkıştırma algoritmalarıyla küçültür ve akabinde 
uçtan uca şifreleyerek yayına sokar. Paket kayıplarına karşı FEC (Forward Error 
Correction) mekanizmaları barındırır.
DURAK: Veri Paketlerinin Bekletilmesi
Ağ üzerinde birden fazla veri tipi akarken, hepsinin aciliyeti aynı değildir. DURAK, 
kısıtlı bant genişliği ortamlarında veri paketlerinin öncelik sırasına göre 
bekletilmesini veya geçirilmesini sağlayan Kalite Yönetimi (Quality of Service - QoS) 
yazılımıdır. Hedef vurma onayı veya düşman saldırı alarmı gibi hayati veriler, yüksek 
çözünürlüklü bir dron kamerası akışından daha yüksek önceliğe atanarak sıranın en 
önüne geçirilir.
TUTUM: Batarya ve Kaynak Tüketimi İzleyicisi
Sahada faaliyet gösteren her türlü elektrikli platformun (dronlar, telsizler, otonom 
yer araçları) enerji durumunu izler. Lineer regresyon ve yapay sinir ağları modelleri 
kullanılarak mevcut batarya tüketim hızları analiz edilir; aracın rüzgara karşı 
uçması veya yokuş çıkması gibi faktörler hesaba katılarak, enerjisi kritik seviyeye 
inen platformlar için sistem otomatik geri dönüş rotaları veya görev devri uyarıları 
üretir.
Faz 2: GÖRGÜ Kümesi (Algılama ve İstihbarat)
ULAK iletişim altyapısı ve veri boru hatları sağlama alındıktan sonra, sistemin dış 
dünyayı algılayacak gözü ve kulağı olan GÖRGÜ kümesi inşa edilir. Bu kümenin amacı, 
sahadan gelen anlamsız piksel, ses veya radyo frekansı yığınlarını alıp, komuta 
kademesinin kullanabileceği yapılandırılmış (structured) istihbarat verisine 
dönüştürmektir.
TANI: Nesne Tespit ve Sınıflandırma
GÖRGÜ kümesinin temel görsel korteksini oluşturan TANI modülü, sahadaki yüksek 
çözünürlüklü video akışlarını milisaniyeler içerisinde işleyerek hedefleri otonom 
olarak sınıflandırmak üzere tasarlanmıştır. Bu modülün mimarisinde, eşzamanlı ve 
yüksek performanslı nesne tespiti yetenekleriyle öne çıkan YOLO (You Only Look Once) 
serisi algoritmaların (YOLOv7 vb.) en güncel sürümleri entegre edilmektedir.23 TANI, 
dinamik savaş alanındaki dost birlikleri, düşman zırhlılarını ve sivil unsurları 
ayırt edebilen çok katmanlı bir evrişimli sinir ağı (CNN) altyapısına sahiptir. 
Sınıflandırılan her nesne, koordinat ve hız vektörü bilgileriyle birlikte doğrudan 
KÜTÜK veritabanına kaydedilir.
KÖZ: Termal İz ve Motor Sıcaklığı Analizi
Kızılötesi (IR) ve termal sensörlerden alınan görüntüleri işleyen özel bir 
görüntüleme modülüdür. Düşman araçlarının motorlarının ısınma veya soğuma 
periyotları, füzelerin aerodinamik sürtünme ısıları veya ağaç altına gizlenmiş 
jeneratörlerin termal imzaları bu modül tarafından analiz edilir. TAITherm veya 
Microsanj sistemlerine benzer termoreflektans görüntüleme (thermoreflectance imaging) 
teorilerinden ilham alan algoritmalarla, ortam ısısı ile hedef ısısı arasındaki mikro 
sapmalar matematiksel olarak hesaplanıp görselleştirilir.24
SEZİ: Doku Analiziyle Kamuflaj Bulma
Geleneksel kenar algılama (edge detection) yöntemlerini yanıltmak için tasarlanmış 
modern askeri kamuflaj ağlarını kırmak için geliştirilmiştir. SEZİ modülü, Gabor 
dalgacıkları (wavelets) ve Darg operatörü yardımıyla hedef ile arka plan arasındaki 
dokusal (texture) uyumsuzlukları analiz eder.27 Üç boyutlu dışbükey (convex) ve 
içbükey (concave) nesnelerin doğada bulunmayan istatistiksel varyans modelleri 
çıkarılarak, ağaçlık veya kayalık arazilerde gizlenmiş hedefler doku farklılığından 
ifşa edilir.30
YANKI: Akustik Konumlandırma
Savaş alanına dağıtılmış mikrofon dizilimlerinden (microphone arrays) gelen ham ses 
verilerini kullanarak silah atışının kaynağını hassas bir şekilde bulur. Yazılım, 
çapraz korelasyon (cross-correlation) ve eşleştirilmiş filtre (matched filter) 
algoritmalarını paralel işlemcilerde çalıştırarak havadaki merminin şok dalgası ve 
namlu ağzı patlamasının (muzzle blast) varış zamanlarını (Time of Arrival - TOA) 
çoklu noktalardan hesaplar.32 Bu üçgenleme (triangulation) yöntemiyle gizlenmiş 
düşman unsurlarının üç boyutlu koordinatları belirlenir.35
DUYUM: Açık Kaynak İstihbarat (OSINT)
Savaşın sadece fiziksel sahada değil, siber ve enformasyon alanında da sürdüğü 
gerçeğiyle hareket eden DUYUM, açık kaynak veriyi anlamlandırır. Sosyal medya 
beslemeleri, yabancı haber siteleri, uydu görüntüleri ve web forumları üzerinden 
Doğal Dil İşleme (NLP) ve Büyük Dil Modelleri (LLM) kullanılarak semantik veri 
madenciliği yapılır.37 Doğrudan etkileşime girmeden, belirli coğrafi bölgelerdeki 
askeri yığınak söylentileri çıkarılarak eyleme dönüştürülebilir (actionable) bir 
istihbarat raporu haline getirilir.40
BAŞKALA: Uydu Fotoğraflarındaki Değişimleri Bulma
Periyodik olarak elde edilen Optik veya Sentetik Açıklı Radar (SAR) uydu fotoğrafları 
üzerinde zaman serisi analizi yapan değişim tespiti (change detection) 
algoritmalarını içerir. İki farklı zaman diliminde çekilmiş görüntülerdeki siper 
kazılması, bina yıkımı veya araç hareketi gibi geometrik değişimler, TransUNet++ 
mimarisi ve modifiye edilmiş çok zamanlı (multitemporal) REACTIV modelleri ile derin 
öğrenme algoritmaları sayesinde otonom olarak bulunur.41 Radar dalgalarının bulutları 
delme yeteneği (SAR) ile optik netlik birleştirilerek hatalı alarmlar minimize 
edilir.44
SÖZGEÇ: Karmaşık Sinyal Ayıklama
Elektronik harp ortamındaki karmaşık sinyal çorbasından, dost birimlere ait 
frekansları ayrıştıran ve arkaplan gürültüsünü dijital sinyal işleme (DSP) filtreleri 
ile temizleyen yazılım modülüdür.46 Spektral analiz yaparak birbirine karışmış olan 
haberleşme sinyallerini, Fourier dönüşümleri (FFT) ve dalgacık dönüşümleri 
aracılığıyla anlamlı frekans bantlarına ayırır ve istihbarat birimlerinin 
incelemesine sunar.
İZLEK: Rota Tahmini
Hareket halindeki bir hedefin (düşman konvoyu, zırhlı birlik veya gemi vb.) KÜTÜK 
üzerinden çekilen geçmiş hız ve konum verilerini alarak, hedefin belirli bir süre 
sonraki muhtemel konumunu tahmin eden kinematik analiz modülüdür. Gelişmiş Kalman 
filtreleri ve Markov zincirleri kullanılarak hedefin ivme değişimleri analiz edilir 
ve topçu birlikleri için "önleme atışı" yapılacak olan projeksiyon koordinatları 
hesaplanır.
KATMAN: Sensör Füzyonu
Sahadaki tekil kamera, radar ve Lidar'dan gelen veriler bireysel olarak parazitli 
veya eksik olabilir. KATMAN, bu heterojen sensörlerden gelen asenkron verileri aynı 
koordinat uzayında üst üste bindirir. Geleneksel Genişletilmiş Kalman Filtresi (EKF) 
yerine, doğrusal olmayan hataları (non-linearities) daha iyi tolere eden Faktör 
Grafikleri (Factor Graph Optimization) ve çok modlu (multi-modal) sensör füzyon 
mimarileri kullanılarak sahanın mutlak ve gecikmesiz bir dijital durumu 
oluşturulur.47
AYDIN: Düşük Işık ve Gece Görüş Netleştirme
Gece operasyonlarında veya sisli/kötü hava şartlarındaki düşük ışıklı (low-light) 
görüntülerin iyileştirilmesini sağlar. İki aşamalı Denoising Diffusion Modelleri 
(DDM) ve konvolüsyonel Retinex ayrıştırma teknikleri sayesinde, sensörden gelen 
piksellerdeki gürültü (noise) temizlenir ve renk sapmaları düzeltilir.52 Karanlık 
bölgelerdeki piksellerin parlaklığı donanım gerektirmeden, tamamen yazılımsal 
algoritmalarla yapay zeka aracılığıyla aydınlatılır.55
GÖLGE: Optik Parlama Tespiti
Ağır şekilde kamufle edilmiş keskin nişancıları veya gözetleme timlerini tespit etmek 
için düşman dürbünlerinden yansıyan ufak optik parlamaları (glint) tespit eder. 
Kameralara entegre dijital sinyal işleme algoritmaları ve otomatik tarama modları 
ile, güneşte parlayan bir cam kırığı veya su birikintisiyle, bir keskin nişancı 
dürbününden seken foton yansıması arasındaki spektral farklar ayrıştırılarak 
saptanır.57
UFKUN: Uzak Mesafe Siluet Analizi
Kamera çözünürlüğünün yetersiz kaldığı çok uzak mesafelerde piksellerin silikleştiği 
durumlarda hedefin cinsini belirler. K-means kümeleme (clustering) teknikleri ve 
makine öğrenimi siluet skorlama algoritmaları kullanılarak, sadece birkaç on 
piksellik silik kümelerden objenin bir kaya mı, bir zırhlı araç mı yoksa çömelmiş bir 
insan profili mi olduğu matematiksel olasılıklarla hesaplanır.61
Faz 3: BUYRUK Kümesi (Stratejik Karar ve Yapay Zeka)
GÖRGÜ tarafından toplanan ve temizlenen devasa veri setleri, BUYRUK kümesinde 
işlenerek taktiksel karar mekanizmalarına dönüştürülür. Sistemin en zeki katmanı olan 
bu küme, savaşın komuta beynidir. Karmaşık operasyonel şartlarda komutana saniyeler 
içinde karar alternatifleri sunarak bilişsel yükü devralır.
TÖRE: Önceliklendirme Ana Algoritması
Savaş sahasında aynı anda yüzlerce hedef (tanklar, piyadeler, radarlar) tespit 
edildiğinde, hangisinin ilk olarak imha edilmesi gerektiğini belirleyen hiyerarşik 
değerlendirme katmanıdır. Çok kriterli karar verme mekanizması olan Analitik 
Hiyerarşi Süreci (Analytic Hierarchy Process - AHP) ve çok nitelikli fayda teorisi 
(multi-attribute utility theory) kullanılarak hedeflerin mesafe, hasar potansiyeli ve 
stratejik önemi gibi parametreleri üzerinden matematiksel bir öncelik sırası 
oluşturulur.64
TAYIN: Lojistik ve Enerji Optimizasyonu
Modern orduların en kırılgan noktası olan mühimmat ve yakıt tedarik zincirini 
yönetir. Yapay zeka tahmine dayalı bakım (predictive maintenance) ve makine öğrenimi 
destekli tedarik zinciri risk yönetimi modelleriyle operasyonel darboğazları 
öngörür.67 Uçuş ve taşıma kargolarının boş bacak (empty leg) maliyetlerini düşürmek 
için bulut tabanlı algoritmalarla lojistik rotalarını anlık optimize ederek 
birliklere en hızlı ikmali sağlar.70
ÖNGÖRÜ: Oyun Teorisi
Düşmanın bir sonraki taktiksel adımını tahmin etmeye yarayan analitik bir satranç 
motorudur. "Sıfır Toplamlı Oyunlar" (Zero-Sum Games) konseptine dayalı olarak, 
Minimax arama algoritmaları, Nim-sum mantığı ve Nash Dengesi hesaplamaları ile 
düşmanın en olası senaryoları simüle edilir.72 Geçmiş veriler ışığında karar ağaçları 
(game trees) kurarak düşman komutanın muhtemel hamlelerini olasılık dağılımıyla 
hesaplar.75
VİCDAN: Atış Onay Mekanizması
Otonom silahların kullanımı sırasında oluşabilecek insan hakları ihlallerini ve sivil 
zayiatını önlemek üzere tasarlanmış etik sınırlandırma modülüdür. "Çifte Etki 
Doktrini" (Doctrine of Double-Effect) ve Cenevre Sözleşmesi kurallarını algoritmik 
bir çerçeveye oturtan bu modül, hedefin etrafındaki sivil varlığını yüzdesel olarak 
analiz eder.76 Hedefte istenmeyen sivil zayiat riski yüksekse, yapay zeka sistemin 
ateşleme yetkisini bloke ederek son kararı "döngüdeki insan"a (human-in-the-loop) 
devreder.79
DİLMAÇ: Emirleri Koda Çevirme
Komutanın operasyon stresi altında doğal dilde sesli olarak veya serbest metin 
formatında verdiği ("Tepenin arkasındaki iki zırhlıyı dronlarla imha et") emirleri, 
otonom sistemlerin anlayabileceği kesin koordinat, rota ve eylem komutlarına 
dönüştüren Büyük Dil Modeli (LLM) tabanlı bir semantik çeviricidir.
BİLGE: Geçmişten Öğrenme Modülü
Sistemin kümülatif hafızasını ve tecrübesini barındırır. Derin Pekiştirmeli Öğrenme 
(Deep Reinforcement Learning) metotlarıyla, gerçekleşen her operasyonun başarısını 
veya başarısızlığını (ödül ve ceza mekanizması) veri setlerine ekler.81 Zamanla 
ağırlıkları güncellenen sinir ağı, algoritmanın bir sonraki görevde daha isabetli 
taktiksel kararlar vermesi için hiper-parametre güncellemeleri gerçekleştirir.
YARGI: Hedef Kimlik Tespiti Son Karar
GÖRGÜ kümesinden (optik, termal, akustik) gelen heterojen verileri birleştirerek, 
şüpheli bir objenin kesin olarak dost, düşman veya sivil olduğuna karar verir. 
Olasılıklı Bilgi Grafikleri (Probabilistic Knowledge Graphs) ve Graf Sinir Ağları 
(GNN) kullanılarak nesnenin sadece görüntüsü değil, ağ üzerindeki geçmiş hareketleri 
ve veritabanı tutarlılığı da değerlendirilerek yüzdelik oranla kimlik teşhisi 
yapılır.82
DÜZEN: Dağınık Birlikleri Yönetme
Coğrafi olarak birbirinden kopuk durumdaki insansız hava, kara ve deniz araçlarının 
operasyonlarını tek bir komuta ekranından koordine eder. RAFT gibi dağıtık konsensüs 
mekanizmalarına (distributed consensus) dayanan algoritmalar sayesinde, iletişim 
altyapısında kopmalar yaşansa bile lider (commander) dronun veya düğümün (node) 
komutayı devralarak sistemin hiyerarşik çöküşünü önlemesini sağlar.10
AKIL: Kaynak Dağıtımı Optimizasyonu
Eldeki kısıtlı silah, füze ve mühimmat kaynaklarını belirlenen hedeflere matematiksel 
olarak en verimli şekilde paylaştıran algoritmik modüldür. Doğrusal Programlama 
(Linear Programming) ve Hedef Tahsis Problemi (Target Allocation Problem) çözücü 
denklemleri çalıştırılarak, hedefin tehdit seviyesine uygun tipte ve asgari miktarda 
cephane harcanması güvence altına alınır.81
SORGUN: Veri Tutarsızlıkları Denetçisi
Sahadan akan devasa veri setlerindeki çelişkileri (örneğin; aynı plakalara sahip iki 
farklı düşman aracının aynı saniyede haritada birbirinden kilometrelerce uzakta 
görünmesi) tespit eder. Olasılıklı varlık bağlama (probabilistic entity linkage) 
algoritmalarıyla çalışarak sahte, hatalı veya arızalı sensörleri tespit eder ve 
bunları sistemden izole ederek komuta kademesinin yanıltılmasını engeller.88
YAZIT: Savaşın Dijital Kaydı
Operasyon sonrasında veya uluslararası hukuki denetimlerde kullanılmak üzere, savaş 
sırasındaki alınan her otonom kararı, ateşleme emrini ve sensör logunu Blockchain 
(Blokzincir) tabanlı dağıtık defter teknolojisi (Distributed Ledger) kullanarak 
şifreler.89 Bu sayede yetkisiz silme, değiştirme veya hackleme girişimlerine karşı 
tam bir "Değişmez Loglama" (Immutable Logging) ve şeffaf denetim ortamı 
oluşturulur.91
KESİN: İhtimal Analizörü
Komutana ham veri veya aşırı teknik detay boğuntusu yaşatmak yerine, belirlenen 
hedefler karşısındaki muhtemel saldırı senaryolarının başarı ihtimallerini analiz 
eder. Bayezyen ağlar (Bayesian Networks) üzerinden çalışarak her bir varyasyonun 
zayiat, süre ve başarı ihtimalini yüzdesel istatistiklere döker ve komutana en 
optimal 3 karar opsiyonunu listeler.
Faz 4: KOVAN Kümesi (Sürü ve Otonom Navigasyon)
Veriler toplandıktan ve kararlar stratejik bir çerçeveye oturtulduktan sonra bu 
kararların fiziksel eyleme dönüşmesi için KOVAN kümesi devreye girer. Bu küme, 
herhangi bir özel donanım üreticisine bağlı kalmaksızın, API üzerinden kontrol 
edilebilen tüm insansız araçları bir "akıllı sürüye" çeviren otonom navigasyon ve 
hareket algoritmalarını kapsar.
SAĞANAK: Sürü (Swarm) Uçuş Mantığı
Birden fazla insansız aracın merkezi bir liderin yönlendirmesi olmadan, tamamen kendi 
aralarındaki iletişimle birlikte hareket etmesini sağlayan swarm algoritmasıdır. 
Yapay Potansiyel Alanlar (Artificial Potential Fields) teorisi ve Derin Pekiştirmeli 
Öğrenme algoritmaları (örn: DQN-SISA) kullanılarak, dronların havada birbirine 
çarpmadan görev bölüşümü yapması, hedefe eşgüdümlü uçması ve dinamik tehditlere karşı 
konum alması sağlanır.94
YERLEM: GPS Olmadan Konum Bulma
Düşmanın elektronik harp yoluyla uyduları kör ettiği veya GPS sinyallerini kestiği 
ortamlarda (GPS-denied environments) araçların navigasyonunu sürdürmesini sağlar. 
Visual SLAM (Simultaneous Localization and Mapping - Eşzamanlı Konumlandırma ve 
Haritalama) algoritmaları, sadece üzerlerindeki tekdüze monoküler veya RGB-D 
kameralardan gelen pikselleri analiz ederek çevrenin dolaylı veya doğrudan 
(indirect/direct approach) haritasını çıkarır ve aracın haritadaki mutlak pozisyonunu 
hesaplar.98
ONAR: Otomatik Görev Devri
Sürü içerisindeki bir veya birden fazla hava aracı vurulduğunda, düştüğünde ya da ağ 
koptuğunda, sürüdeki diğer araçların eksilen yükü derhal üstlenmesini sağlayan 
dinamik optimizasyon yazılımıdır. Araçlar arası konumlandırmayı radyo sessizliğinde 
sağlamak için UVDAR (Ultra-Violet Direction and Ranging) benzeri görsel/ultraviyole 
led iletişim metotları kullanılarak eksilen birimlerin hedefleri milisaniyeler 
içerisinde sağ kalan birimlere devredilir.102
EVRİM: Formasyon Değiştirme
Görev sırasında hava veya kara şartlarına göre sürünün fiziksel dizilimini değiştiren 
adaptif kontrol yazılımıdır. Önlerine dar bir vadi, dar bir sokak veya düşman hava 
savunma sistemi çıktığında, dağınık sürü düzeni anlık olarak karar alıp V 
formasyonuna veya tek sıra (file) düzenine geçiş yapar; tehlike geçince tekrar otonom 
şekilde eski dağılımına evrilir.
SAPMA: Balistik Hesaplayıcı
Mühimmat kullanımından önce çevresel değişkenleri hesaba katan yazılımsal hesaplama 
modülüdür. Lazer mesafe ölçücülerden ve anemometrelerden gelen verileri girdi olarak 
kabul ederek, rüzgar hızı, havadaki nem, barometrik basınç, namlu çıkış hızı ve 
mermiye etki eden Coriolis kuvveti gibi değişkenleri doğrusal olmayan diferansiyel 
denklemlerle çözer ve atış sistemlerine hedefleme ofsetini (sapma payı) bildirir.
KOŞUT: Kara ve Hava Eşzamanlı Hareketi
Salt hava unsurlarını değil, insansız kara araçlarının (İKA) da havadaki dronlarla 
"Marsupial" (keseli) robotik mantığında tam senkronize ilerlemesini sağlar. DDPG 
(Deep Deterministic Policy Gradient) gibi pekiştirmeli öğrenme modelleri 
çalıştırılarak, havadaki dronun haritaladığı geniş alan verisi, yerdeki aracın önüne 
çıkacak kör nokta engellerini aşması için kılavuz olarak kullanılır.106
DENGEL: Uçuş ve Sürüş Stabilizasyonu
Aşırı rüzgar, şiddetli sağanak yağmur, veya yakında gerçekleşen patlama kaynaklı şok 
dalgaları karşısında aracın aniden devrilmesini, düşmesini engellemek için PID 
(Proportional-Integral-Derivative) kontrol döngülerini milisaniyeler içinde optimize 
eden matematiksel denge algoritmasıdır. Fizik motoru simülasyonları gibi çalışarak 
motor torklarını anlık revize eder.
BİÇİM: 3D Harita Otomatik Rota Çizimi
Şehir içi harp gibi binalarla dolu karmaşık 3D ortamlarda (urban environments), 
hedefe giden en hızlı ve güvenli yolu hesaplar. Klasik iki boyutlu A* algoritmaları 
yerine, ortamı yüksek çözünürlüklü küplere bölen Voksel (Voxel) modelleme mimarisi ve 
geliştirilmiş Boustrophedon Hücre Ayrıştırması (BCD) yöntemlerini kullanarak, 
dronların binalar arasından birbirleriyle çakışmayan Çoklu Gezgin Satıcı (MTSP) 
mantığında 3D rotalar çizmesini sağlar.110
SADAK: Mühimmat Kullanım Verimliliği
Atış sistemlerindeki sensörler ile hedefin mesafesini, hızını ve zırh tipini analiz 
ederek, yapay zeka destekli hedefleme algoritmalarıyla (örneğin AimLock CTM benzeri 
115) kullanılacak mühimmat cinsini ve sayısını seçer. Boşa mermi atılmasını veya 
yumuşak bir hedefe anti-tank füzesi harcanmasını engelleyen akıllı mühimmat lojistik 
kontrol (smart logistics) kod bloğudur.116
ÇEVİK: Ani Manevra Kod Bloğu
Aracın kör noktasından aniden önüne fırlayan bir objeyi, kuşu veya kendisine doğru 
yaklaşan füzeyi lidar ve kameralar aracılığıyla algılayıp, ana sistemden onay 
beklemeye gerek kalmadan saliseler içinde (edge computing tepkimesiyle) çarpışmayı 
önleyici manevralar üreten otonom kaçınma fonksiyonudur.
YÖNEL: Manyetik Pusula Sapması Düzenleyici
Geleneksel pusulalar, dünyanın manyetik alanındaki düzensizlikler ve hava aracındaki 
elektronik cihazların yarattığı "Hard Iron / Soft Iron" etkileri nedeniyle sürekli 
saparlar. YÖNEL modülü, Dünya Manyetik Modeli (WMM) algoritmalarını ve filtrelenmiş 
GPS verilerini kullanarak manyetik sapmayı (declination/deviation) hesaplar ve 
navigasyon rotasını tamamen yazılımsal olarak otonom kalibre eder.120
ÖNCÜ: Keşif Birimi Mantığı
Sürünün ana gövdesinden önde giden, hedefini imha etmek yerine salt keşif amacı güden 
hafif dronlara özel yüklenen navigasyon doktrinidir. Düşman radarına veya termal 
görüşe yakalanmamak için radyo sinyali yaymadan (radyo sessizliği) pasif olarak arazi 
örtüsünün arkasından (nap-of-the-earth) süzülme uçuşu yapmayı sağlayan otonom hareket 
kodudur.
Faz 5: KIN Kümesi (Elektronik Harp ve Siber Savunma)
Büyük ve otonom bir beyin inşa edildikten sonra, en kritik süreç bu yapıyı düşmanın 
görünmez saldırılarına karşı korumaktır. KIN kümesi, elektromanyetik spektrumun, 
siber dünyanın ve frekans dalgalarının yönetildiği hem savunma (kalkan) hem de 
elektronik taarruz merkezidir.
AYIK: Sinyal Karıştırmayı (Jamming) Fark Etme
Düşman tarafından sahada uygulanan GPS veya telsiz iletişim karıştırmalarını 
(Jamming) gerçek zamanlı olarak saptayan elektronik savunma sistemidir. Yazılım 
Tanımlı Radyo (SDR) üzerinden gelen IQ verilerini Makine Öğrenimi tabanlı modeller 
(örneğin JamShield yaklaşımı 123) ile analiz ederek, geniş bantlı CDMA veya dar 
bantlı Chirp tarzı karıştırma saldırılarını anında sınıflandırır; dinamik anten huzme 
yönlendirme (beamsteering) algoritmalarıyla gürültü kaynaklarını yazılımsal olarak 
filtreleyip baskılar.124
ÖZEK: Frekans Atlamalı Telsiz Çözücü
Düşman unsurlarının taktiksel muhaberesini dinleyebilmek için geliştirilmiş dijital 
sinyal analiz modülüdür. Saniyede binlerce kez frekans değiştiren FHSS (Frequency 
Hopping Spread Spectrum) sinyallerini taramak üzere SDR (Software Defined Radio) 
donanımları içindeki Esnek Faz Kilitli Döngü (PLL) mimarilerini yönlendirir.128 Gelen 
sinyallerin zaman-frekans kalıplarını GNU Radio tarzı platformlarla işleyerek, 
atlamalı frekansları tespit eder ve iletişim paketlerini otonom olarak birleştirip 
demodüle eder.130
EŞİK: Ağ Sızma Güvenlik Duvarı
MATRİS iletişim ağına düşman tarafından dışarıdan yapılabilecek izinsiz sızmaları ağ 
paket düzeyinde inceleyen Derin Paket İnceleme (Deep Packet Inspection) mimarisidir. 
Yapay zeka ile ağ üzerindeki normal trafik alışkanlıklarını öğrenir, olağandışı veri 
trafiği saptandığında bağlantıyı otonom bir şekilde kesen ve mikro-segmentasyonu 
daraltan Zero-Trust tabanlı güvenlik duvarı işlevi görür.
SAĞLAM: GPS Doğrulama ve Düzeltme
Düşman tarafından kuvvetli sinyal yayıcılarla üretilen sahte GPS sinyallerini 
(Spoofing) saptayarak araçların rotadan sapmasını veya yanlış yönlere çekilmesini 
engeller. Gelen uydu sinyallerinin taşıyıcı gücünü, geliş açısını ve Doppler 
kaymasını matematiksel olarak analiz ederek sahte veri üreten kaynakları ayıklar, 
gerekirse navigasyon çözümünü GPS'ten INS'e veya YERLEM (V-SLAM) modülüne 
devreder.126
MÜHÜR: Kuantum Sonrası Veri Şifreleme (PQC)
Geleceğin kuantum bilgisayarlarının günümüzde kullanılan RSA veya Eliptik Eğri (ECC) 
tabanlı algoritmaları saniyeler içinde kırabilme tehdidine karşı sistemi garanti 
altına alır. NIST tarafından standardize edilen ML-KEM (eski adıyla Kyber) gibi 
kuantum dirençli anahtar değişim mekanizmaları ve ML-DSA (Dilithium) dijital imza 
algoritmaları kullanılarak, Rust ve C++ dilleriyle yüksek hızlı ve "Post-Quantum" 
seviyesinde kırılmaz veri tünelleri yaratır.133
SIR: Görünmez Haberleşme Protokolü
Low Probability of Intercept / Low Probability of Detection (LPI/LPD) konseptine 
dayalı gizli haberleşme modülüdür. Spatial Pilot Perturbation (SPP) veya yayılı 
spektrum benzeri teknikler kullanılarak, drone'lar veya birlikler arası iletişim 
sinyallerinin gücünü çevresel elektronik gürültü seviyesinin altına (noise floor) 
bastırır.137 Düşman elektronik istihbarat (ELINT) sistemleri bu iletişimi yalnızca 
bir arka plan cızırtısı olarak algılar ve deşifre edemez.140
YALVAÇ: Sahte Sinyal (Spoofing) Üretimi
Elektronik harpte savunmanın ötesine geçerek aktif yanıltma yapan modüldür. Yazılım 
tabanlı sahte eko, doppler ve GPS konum verileri yaratarak, tek bir gözetleme 
dronunun yaydığı sinyalleri düşman radarında koca bir savaş uçağı filosu 
yaklaşıyormuş gibi gösterir ve düşmanın füzelerini veya dikkatini yanlış bölgelere 
kanalize eder.
KIRIM: Siber Saldırı ve Sızma Ajanları
Düşmanın komuta kontrol ağlarına ve sunucularına otonom sızmak üzere programlanmış, 
Büyük Dil Modelleri ile entegre çalışan siber taarruz (Agentic LLMs) ajanlarıdır.142 
Tıpkı ALOHA (Agentic LLMs for Offensive Heuristic Automation) sisteminde olduğu gibi, 
düşman sistemlerindeki açık kapıları, zayıf CVE protokollerini ve zafiyetleri otonom 
olarak tarayarak, saatler sürecek penetrasyon ve sızma operasyonlarını insan 
müdahalesiz saniyelere indirger, gerektiğinde karşı sistemleri felç eder.144
KORAY: Yazılımsal Radar İzi (RCS) Küçültme
Hava araçlarının Radar Kesit Alanını (RCS) fiziksel şekil değişikliğiyle veya RAM 
(Radar Absorbing Material) kaplamayla azaltmanın mümkün olmadığı anlarda, yön ve açı 
hileleriyle radar görünmezliği sağlar. RL-GO (Ray-Launching Geometrical-Optics) veya 
Feko simülasyon algoritmaları kullanılarak aracın radara karşı anlık elektromanyetik 
saçılımı hesaplanır; araç, düşman radar dalgalarına karşı en az yansıma yapacak yatış 
veya burun açısına kendini milisaniyeler içinde konumlandırarak yazılımsal bir 
"stealth" kalkan elde eder.137
ÖRTÜ: Kendi Sistemlerimizin İzini Gizleme
Kendi komuta kontrol merkezlerimizin ve röle iletişim düğümlerinin IP adreslerini, 
MAC adreslerini ve spektrum yayım karakteristiklerini algoritmik bir rotasyonla 
sürekli değiştirip maskeleyerek sistemin gerçek komuta merkezini hem fiziksel radar 
tespitinden hem de siber saldırılardan korur.
GEÇİT: Güvenli Veri Tünelleme
Standart VPN teknolojilerinin ötesinde, sahadaki dronlar ile komuta merkezi arasında 
iletilen her bir bağımsız veri paketi için anlık olarak ayrı asimetrik anahtarlar 
üreterek mikrosaniyelik sanal veri tünelleri kurar. Veri paketi hedefe ulaşır ulaşmaz 
o tünel bir daha kullanılmamak üzere kalıcı olarak imha edilir.
SİBİ: Siber Saldırı Kaynağını İzleme
ÖTÜKEN-MATRİS sistemine yönelik karmaşık bir siber saldırı (DDoS, fidye yazılımı, 
sızma girişimi vb.) gerçekleştiğinde, saldırının botnet ağları veya proxy sunucuları 
üzerinden defalarca sektirildiği IP adreslerini makine hızında geriye dönük izleyen 
(traceback) bir ağ analizörüdür. Gerçek saldırganın coğrafi konumunu ve cihaz ID'sini 
tespit ederek KIRIM modülüne karşı-taarruz için veri besler.
Sonuç
ÖTÜKEN-MATRİS, savaş meydanının kurallarını sadece zırhla veya barutla değil, derin 
matematiksel algoritmalar ve dağıtık yazılım zekası ile baştan yazmayı hedefleyen 
eşsiz bir donanımsız savunma altyapısıdır. Rapor boyunca detaylandırılan 5 faz ve 60 
otonom modül; iletişimin koparılmaz bir sinir ağına (ULAK) dönüştürülmesi, çevrenin 
fotogerçekçi ve sensör füzyonlu tespiti (GÖRGÜ), oyun teorisi ve kuantum hesaplamalar 
içeren bir taktik beynin (BUYRUK) oluşturulması, makinelerin merkezi kontrolsüz ama 
ortak bir organizma gibi hareket etmesi (KOVAN) ve elektronik spektrumun bütünüyle 
hacklenip sistemin görünmez bir kalkanla (KIN) korunması felsefesine dayanmaktadır.
Yazılım mimarisinin donanımdan ayrıştırılarak konteynerize edilmiş modüllere 
bölünmesi, her türlü eski veya modern fiziksel silah platformunun tek bir yazılım 
güncellemesiyle modern ağ merkezli harbe (CJADC2) dahil edilebilmesini sağlayacaktır. 
Bu doküman, koda döküldüğünde çeliği yönetecek beynin teorik yapısını, makine 
öğreniminin en uç yetenekleriyle birlikte savunma endüstrisi profesyonellerinin 
referans alabileceği sağlam bir mühendislik zemini olarak sunmaktadır.
Alıntılanan çalışmalar
C4ISR Systems - BAE Systems, erişim tarihi Mart 4, 2026, 
https://www.baesystems.com/en-us/who-we-are/electronic-systems/c4isr
Industry Study Report - Eisenhower School, erişim tarihi Mart 4, 2026, 
https://es.ndu.edu/Portals/75/Documents/industry-study/reports/2023/AY23%20C4ISR-Cleared.pdf
Zero Trust Architecture: A Systematic Literature Review - arXiv.org, erişim tarihi 
Mart 4, 2026, https://arxiv.org/html/2503.11659v2
What Is Zero Trust Architecture? Key Elements and Use Cases - Palo Alto Networks, 
erişim tarihi Mart 4, 2026, 
https://www.paloaltonetworks.com/cyberpedia/what-is-a-zero-trust-architecture
Zero-trust security: What architects need to know - Red Hat, erişim tarihi Mart 4, 
2026, https://www.redhat.com/en/blog/what-is-zero-trust
Department of Defense Zero Trust Reference Architecture - DoD CIO, erişim tarihi Mart 
4, 2026, 
https://dodcio.defense.gov/Portals/0/Documents/Library/(U)ZT_RA_v2.0(U)_Sep22.pdf
Architecture | OpenYurt, erişim tarihi Mart 4, 2026, 
https://openyurt.io/docs/core-concepts/architecture/
Edge Computing Implementation Strategy: A Complete Guide for Distributed Teams, 
erişim tarihi Mart 4, 2026, 
https://fullscale.io/blog/edge-computing-implementation-strategy/
Designing a Real Time Data Processing System | Subhadip Mitra, erişim tarihi Mart 4, 
2026, 
https://subhadipmitra.com/blog/2021/designing-a-real-time-data-processing-system/
Distributed Consensus Protocols and Algorithms, erişim tarihi Mart 4, 2026, 
https://cybersecurity.seas.wustl.edu/ning/paper/consensus19.pdf
Managing Critical State: Distributed Consensus for Reliability - Google SRE, erişim 
tarihi Mart 4, 2026, https://sre.google/sre-book/managing-critical-state/
Distributed System Design with RAFT Consensus Algorithms - YouTube, erişim tarihi 
Mart 4, 2026, https://www.youtube.com/watch?v=S6mDA8X8drs
Modern database management in distributed systems - Aerospike, erişim tarihi Mart 4, 
2026, https://aerospike.com/blog/modern-database-management-in-distributed-systems/
Rust's Strategic Advantage - sysid blog, erişim tarihi Mart 4, 2026, 
https://sysid.github.io/rusts-strategic-advantage/
Rust vs. C++: a Modern Take on Performance and Safety - The New Stack, erişim tarihi 
Mart 4, 2026, 
https://thenewstack.io/rust-vs-c-a-modern-take-on-performance-and-safety/
Memory Safe Languages: Reducing Vulnerabilities in Modern Software Development, 
erişim tarihi Mart 4, 2026, 
https://media.defense.gov/2025/Jun/23/2003742198/-1/-1/0/CSI_MEMORY_SAFE_LANGUAGES_REDUCING_VULNERABILITIES_IN_MODERN_SOFTWARE_DEVELOPMENT.PDF
The Complete Guide to Edge Computing Architecture | Mirantis, erişim tarihi Mart 4, 
2026, 
https://www.mirantis.com/blog/the-complete-guide-to-edge-computing-architecture/
Store Edge Networked Data (SEND): A Data and Performance Driven Edge Storage 
Framework - PMC, erişim tarihi Mart 4, 2026, 
https://pmc.ncbi.nlm.nih.gov/articles/PMC8341071/
Advanced 3D Visualization Solutions - Skyline Software, erişim tarihi Mart 4, 2026, 
https://www.skylinesoft.com/advanced-visualization-solutions/
Three-Dimensional Multitemporal Game Engine Visualizations for Watershed Analysis, 
Lighting Simulation, and Change Detection in Built Environments - MDPI, erişim tarihi 
Mart 4, 2026, https://www.mdpi.com/2220-9964/14/7/265
Advanced Visualization: Powerful Immersive 3D Software Brings Infrastructure to Life, 
erişim tarihi Mart 4, 2026, https://www.youtube.com/watch?v=snumEKbmGxQ
WDT 2024 3D Science Visualization - The University of Rhode Island, erişim tarihi 
Mart 4, 2026, https://web.uri.edu/riinbre/3d-science-visualization_wdt2024/
12 of the Most Popular Computer Vision Tools - Viso Suite, erişim tarihi Mart 4, 
2026, https://viso.ai/computer-vision/the-most-popular-computer-vision-tools/
Aerospace | ThermoAnalytics, erişim tarihi Mart 4, 2026, 
https://www.thermoanalytics.com/aerospace
Microsanj Products Overview, erişim tarihi Mart 4, 2026, 
https://microsanj.com/products/products-overview/
Intelligent Thermal Imaging-Based Diagnostics of Turbojet Engines - MDPI, erişim 
tarihi Mart 4, 2026, https://www.mdpi.com/2076-3417/9/11/2253
Assessing Camouflage Methods Using Textural Features - DTIC, erişim tarihi Mart 4, 
2026, https://apps.dtic.mil/sti/tr/pdf/ADP010540.pdf
Method of camouflage evaluation based on texture analysis model of Gabor wavelet, 
erişim tarihi Mart 4, 2026, 
https://www.researchgate.net/publication/291290568_Method_of_camouflage_evaluation_based_on_texture_analysis_model_of_Gabor_wavelet
Computer vision, camouflage breaking and countershading - PMC, erişim tarihi Mart 4, 
2026, https://pmc.ncbi.nlm.nih.gov/articles/PMC2674074/
A Survey of Camouflaged Object Detection and Beyond - arXiv.org, erişim tarihi Mart 
4, 2026, https://arxiv.org/html/2408.14562v1
Image Analysis and Processing for Generating Camouflages from Digital Earth 
Photographs, erişim tarihi Mart 4, 2026, https://www.mdpi.com/2076-3417/13/1/403
FPGA-Based Implementation of a Matched Filter for Acoustic Shooter Localization 
System, erişim tarihi Mart 4, 2026, https://ieeexplore.ieee.org/document/10224025/
Gunshot Detection & Localization System, erişim tarihi Mart 4, 2026, 
https://infsyst.com/systems/gunshot-localization/
Gunshot Location Detection with 3 GRS Units - Crystal Instruments, erişim tarihi Mart 
4, 2026, https://www.crystalinstruments.com/blog/2024/6/19/gunshot-location-detection
Precision and accuracy of acoustic gunshot location in an urban environment - 
SoundThinking, erişim tarihi Mart 4, 2026, 
https://www.soundthinking.com/wp-content/uploads/2021/08/TN-098-Accuracy-of-Acoustic-Gunshot-Location.pdf
Audio Gunshot Detection and Localization Systems: History, Basic Design, and Future 
Possibilities by Jordan R. Graves, erişim tarihi Mart 4, 2026, 
https://www.ucdenver.edu/docs/librariesprovider27/ncmf-docs/theses/graves_thesis_fall2012.pdf?sfvrsn=8e4d97b8_2
From Open Source to Operational Insight: How OSINT Is Shaping Modern Intelligence, 
erişim tarihi Mart 4, 2026, 
https://www.leidos.com/insights/open-source-operational-insight-how-osint-shaping-modern-intelligence
AI-driven situational intelligence in military context | HENSOLDT, erişim tarihi Mart 
4, 2026, 
https://www.hensoldt.net/domains/cyber/Intelligence-solutions/AI-driven-situational-intelligence
Redefining OSINT Software Architecture With System-Centric Architecture Design: A 
Framework Shaped by QAW, ADD, and ATAM - IEEE Xplore, erişim tarihi Mart 4, 2026, 
https://ieeexplore.ieee.org/iel8/6287639/10820123/10971222.pdf
ECS Open Source Intelligence Solutions, erişim tarihi Mart 4, 2026, 
https://ecstech.com/solutions/artificial-intelligence/ecs-open-source-intelligence-solutions/
TransUNet++SAR: Change Detection with Deep Learning about Architectural Ensemble in 
SAR Images - MDPI, erişim tarihi Mart 4, 2026, https://www.mdpi.com/2072-4292/15/1/6
MinZHANG-WHU/Change-Detection-Review: A review of change detection methods, including 
codes and open data sets for deep learning. From paper: change detection based on 
artificial intelligence: state-of-the-art and challenges. - GitHub, erişim tarihi 
Mart 4, 2026, https://github.com/MinZHANG-WHU/Change-Detection-Review
Application of Multitemporal Change Detection in Radar Satellite Imagery Using 
REACTIV-Based Method for Geospatial Intelligence - PMC, erişim tarihi Mart 4, 2026, 
https://pmc.ncbi.nlm.nih.gov/articles/PMC10223237/
Change Detection in Heterogeneous Optical and SAR Remote Sensing Images via Deep 
Homogeneous Feature Fusion - arXiv.org, erişim tarihi Mart 4, 2026, 
https://arxiv.org/pdf/2004.03830
Improved Difference Images for Change Detection Classifiers in SAR Imagery Using Deep 
Learning - IEEE Xplore, erişim tarihi Mart 4, 2026, 
https://ieeexplore.ieee.org/iel7/36/10006360/10286479.pdf
Engineers Guide to Digital Signal Processing - Used Keysight Equipment, erişim tarihi 
Mart 4, 2026, 
https://www.keysight.com/used/us/en/knowledge/guides/digital-signal-processing
Factor Graph Based Sensor Fusion - TU Chemnitz, erişim tarihi Mart 4, 2026, 
https://www.tu-chemnitz.de/etit/proaut/en/research/fg-sf.html
Kalman Filter, Sensor Fusion, and Constrained Regression: Equivalences and Insights - 
UC Berkeley Statistics Department, erişim tarihi Mart 4, 2026, 
https://www.stat.berkeley.edu/~ryantibs/papers/sensorfus.pdf
Multisensor Fusion SLAM: Comprehensive Overview - ALLPCB, erişim tarihi Mart 4, 2026, 
https://www.allpcb.com/allelectrohub/multisensor-fusion-slam-comprehensive-overview
Multi-Modal Sensor Fusion and Object Tracking for Autonomous Racing - Semantic 
Scholar, erişim tarihi Mart 4, 2026, 
https://www.semanticscholar.org/paper/2881bb100551d401f04987d2f2221979f13450bc
[2411.03702] Graph-Based Multi-Modal Sensor Fusion for Autonomous Driving - arXiv, 
erişim tarihi Mart 4, 2026, https://arxiv.org/abs/2411.03702
Two-Stage Denoising Diffusion Model for Low-Light Image Enhancement - MDPI, erişim 
tarihi Mart 4, 2026, https://www.mdpi.com/2076-3417/16/1/18
LECARM: Low-Light Image Enhancement using Camera Response Model - ResearchGate, 
erişim tarihi Mart 4, 2026, 
https://www.researchgate.net/publication/324645690_LECARM_Low-Light_Image_Enhancement_using_Camera_Response_Model
A survey on image enhancement for Low-light images - PMC, erişim tarihi Mart 4, 2026, 
https://pmc.ncbi.nlm.nih.gov/articles/PMC10070385/
royerlab/aydin: Aydin — User-friendly, Fast, Self-Supervised Image Denoising for All. 
- GitHub, erişim tarihi Mart 4, 2026, https://github.com/royerlab/aydin
Research on Low-Light Image Enhancement Algorithm Based on Dual-Branch Structure | 
Francis Academic Press, erişim tarihi Mart 4, 2026, 
https://francis-press.com/papers/18689
Digital SIGINT System (DSS) - Sierra Nevada Corporation | SNC, erişim tarihi Mart 4, 
2026, https://www.sncorp.com/capabilities/digital-sigint-system-dss/
Optical Anti-sniper Detection System Market Trends & Demand 2027, erişim tarihi Mart 
4, 2026, 
https://www.transparencymarketresearch.com/optical-anti-sniper-detection-system-market.html
Performance of glint correction algorithms for Sentinel-3 OLCI data - Frontiers, 
erişim tarihi Mart 4, 2026, 
https://www.frontiersin.org/journals/remote-sensing/articles/10.3389/frsen.2025.1690337/full
GLINT - DTIC, erişim tarihi Mart 4, 2026, 
https://apps.dtic.mil/sti/citations/ADA361747
A review of silhouette extraction algorithms for use within visual hull pipelines, 
erişim tarihi Mart 4, 2026, 
https://shura.shu.ac.uk/26853/3/Choppin_ReviewSillhouetteExtraction%28AM%29.pdf
Silhouette Analysis for Performance Evaluation in Machine Learning with Applications 
to Clustering - MDPI, erişim tarihi Mart 4, 2026, 
https://www.mdpi.com/1099-4300/23/6/759
Silhouette Score, Inliers, and Outliers - YouTube, erişim tarihi Mart 4, 2026, 
https://www.youtube.com/watch?v=a2Kg2_l3L8M
A Value-Based Hierarchy of Objectives for Military Decision-Making - AFIT Scholar, 
erişim tarihi Mart 4, 2026, https://scholar.afit.edu/etd/7839/
(PDF) USE OF ANALYTIC HIERARCHY PROCESS MODELING IN THE MILITARY DECISION MAKING 
PROCESS FOR COURSE OF ACTION EVALUATION AND UNIT COHESION - ResearchGate, erişim 
tarihi Mart 4, 2026, 
https://www.researchgate.net/publication/351446225_USE_OF_ANALYTIC_HIERARCHY_PROCESS_MODELING_IN_THE_MILITARY_DECISION_MAKING_PROCESS_FOR_COURSE_OF_ACTION_EVALUATION_AND_UNIT_COHESION
Decision Analysis with the Analytic Hierarchy Process as a Methodology to Develop a 
Model of Space Mobility Utility - DTIC, erişim tarihi Mart 4, 2026, 
https://apps.dtic.mil/sti/trecms/pdf/AD1178955.pdf
Improving military logistics: using AI and advanced tracking systems to optimise 
operations | by Thomas Sander | Medium, erişim tarihi Mart 4, 2026, 
https://medium.com/@thomasarnosander/improving-military-logistics-using-ai-and-advanced-tracking-systems-to-optimise-operations-a682868ff489
Artificial Intelligence for Military Logistics - Current Applications, erişim tarihi 
Mart 4, 2026, https://emerj.com/artificial-intelligence-military-logistics/
Artificial Intelligence and Agile Combat Employment - Army University Press, erişim 
tarihi Mart 4, 2026, 
https://www.armyupress.army.mil/Journals/Military-Review/English-Edition-Archives/May-June-2024/MJ-24-Hagardt/
AI-Driven Logistics Optimization for Military Operations - Oasis of Ideas, erişim 
tarihi Mart 4, 2026, 
https://www.oasis-of-ideas.com/ideas/ai-driven-logistics-optimization-for-military-operations
Air Logistics Optimization Prototyped for DoD - Defense Innovation Unit, erişim 
tarihi Mart 4, 2026, 
https://www.diu.mil/latest/air-logistics-optimization-prototyped-for-dod-leveraging-enterprise
Some Military Applications of the Theory of Games - RAND, erişim tarihi Mart 4, 2026, 
https://www.rand.org/content/dam/rand/pubs/papers/2006/P1849.pdf
Applied Game Theory to Improve Strategic and Tactical Military Decisions - Longdom 
Publishing, erişim tarihi Mart 4, 2026, 
https://www.longdom.org/open-access-pdfs/applied-game-theory-to-improve-strategic-and-tactical-military-decisions-2167-0374-1000147.pdf
Game Theory and Algorithms: A Comprehensive Exploration | by Rajat Sharma | Medium, 
erişim tarihi Mart 4, 2026, 
https://medium.com/@rajat01221/game-theory-and-algorithms-a-comprehensive-exploration-55e510535302
Nuclear War and Game Theory - Cornell blogs, erişim tarihi Mart 4, 2026, 
https://blogs.cornell.edu/info2040/2021/09/21/nuclear-war-and-game-theory/
An Ethics Framework for Autonomous Weapon Systems - Royal Air Force, erişim tarihi 
Mart 4, 2026, 
https://www.raf.mod.uk/what-we-do/centre-for-air-and-space-power-studies/aspr/aspr-vol22-iss3-3-pdf/
Ethical Imperatives for Lethal Autonomous Weapons - Belfer Center, erişim tarihi Mart 
4, 2026, 
https://www.belfercenter.org/publication/ethical-imperatives-lethal-autonomous-weapons
A Hazard to Human Rights: Autonomous Weapons Systems and Digital Decision-Making, 
erişim tarihi Mart 4, 2026, 
https://www.hrw.org/report/2025/04/28/a-hazard-to-human-rights/autonomous-weapons-systems-and-digital-decision-making
Ethics and autonomous weapon systems: An ethical basis for human control? - ICRC, 
erişim tarihi Mart 4, 2026, 
https://www.icrc.org/en/download/file/69961/icrc_ethics_and_autonomous_weapon_systems_report_3_april_2018.pdf
Balancing Power and Ethics: A Framework for Addressing Human Rights Concerns in 
Military AI - arXiv.org, erişim tarihi Mart 4, 2026, 
https://arxiv.org/html/2411.06336v1
Understanding the Limits of Artificial Intelligence for Warfighters - RAND 
Corporation, erişim tarihi Mart 4, 2026, 
https://www.rand.org/pubs/research_reports/RRA1722-5.html
A probabilistic knowledge graph for target identification - PMC - NIH, erişim tarihi 
Mart 4, 2026, https://pmc.ncbi.nlm.nih.gov/articles/PMC11034645/
Probabilistic Search for Tracking Targets: Theory and Modern Applications | Wiley, 
erişim tarihi Mart 4, 2026, 
https://www.wiley.com/en-us/Probabilistic+Search+for+Tracking+Targets%3A+Theory+and+Modern+Applications-p-9781118596593
A probabilistic knowledge graph for target identification | PLOS Computational 
Biology, erişim tarihi Mart 4, 2026, 
https://journals.plos.org/ploscompbiol/article?id=10.1371/journal.pcbi.1011945
Analysis of a Distributed Command-and-Control Algorithm to Implement Mosaic Warfare, 
erişim tarihi Mart 4, 2026, 
https://www.researchgate.net/publication/384209231_Analysis_of_a_Distributed_Command-and-Control_Algorithm_to_Implement_Mosaic_Warfare
AI-Driven Decision Support Systems in Agile Software Project Management: Enhancing 
Risk Mitigation and Resource Allocation - MDPI, erişim tarihi Mart 4, 2026, 
https://www.mdpi.com/2079-8954/13/3/208
A Linear Programming Approach to Weapon Allocation - DTIC, erişim tarihi Mart 4, 
2026, https://apps.dtic.mil/sti/tr/pdf/AD0721458.pdf
A Probabilistic Data Fusion Modeling Approach for Extracting True Values from 
Uncertain and Conflicting Attributes - MDPI, erişim tarihi Mart 4, 2026, 
https://www.mdpi.com/2504-2289/6/4/114
Immutable Data Logging Using Blockchain for Regulatory Compliance - ResearchGate, 
erişim tarihi Mart 4, 2026, 
https://www.researchgate.net/publication/397481500_Immutable_Data_Logging_Using_Blockchain_for_Regulatory_Compliance
Blockchain Applications in the Military Domain: A Systematic Review - MDPI, erişim 
tarihi Mart 4, 2026, https://www.mdpi.com/2227-7080/13/1/23
Blockchain and Immutable Logging for Audit Integrity - LogZilla, erişim tarihi Mart 
4, 2026, https://www.logzilla.ai/blogs/blockchain-log-management-immutable-logging
Blockchain Applications in the Military Domain: A Systematic Review - Preprints.org, 
erişim tarihi Mart 4, 2026, https://www.preprints.org/manuscript/202411.2266
Using Blockchain Technology to Help Secure America's Defense Critical Infrastructure 
- Digital Commons @ USF - University of South Florida, erişim tarihi Mart 4, 2026, 
https://digitalcommons.usf.edu/cgi/viewcontent.cgi?article=1131&context=mca
Multi-Agent Systems and Swarm Intelligence for Autonomous Drone Coordination | 
Scientific Research Journal of Engineering and Computer Sciences - iarconsortium, 
erişim tarihi Mart 4, 2026, 
https://www.iarconsortium.org/srjecs/178/2899/multi-agent-systems-and-swarm-intelligence-for-autonomous-drone-coordination-4985/
Intelligent Scheduling Technology of Swarm Intelligence Algorithm for Drone Path 
Planning, erişim tarihi Mart 4, 2026, https://www.mdpi.com/2504-446X/8/4/120
Swarm Intelligence Path-Planning Pipeline and Algorithms for UAVs: Simulation, 
Analysis and Recommendation - SciTePress, erişim tarihi Mart 4, 2026, 
https://www.scitepress.org/Papers/2024/126860/126860.pdf
Autonomous Drone Swarm Navigation and Multi-target Tracking in 3D Environments with 
Dynamic Obstacles - arXiv, erişim tarihi Mart 4, 2026, 
https://arxiv.org/pdf/2202.06253
Development of an Online Adaptive Parameter Tuning vSLAM Algorithm for UAVs in 
GPS-Denied Environments - PMC, erişim tarihi Mart 4, 2026, 
https://pmc.ncbi.nlm.nih.gov/articles/PMC9610021/
Evaluating the Efficiency and Accuracy of vSLAM Systems for UAVs in GPS-Denied 
Conditions - Diva-Portal.org, erişim tarihi Mart 4, 2026, 
http://www.diva-portal.org/smash/get/diva2:1884353/FULLTEXT01.pdf
A Multi-Sensorial Simultaneous Localization and Mapping (SLAM) System for Low-Cost 
Micro Aerial Vehicles in GPS-Denied Environments - MDPI, erişim tarihi Mart 4, 2026, 
https://www.mdpi.com/1424-8220/17/4/802
Vision-based Localization: A Guide to VBL Techniques for GPS-denied Environments, 
erişim tarihi Mart 4, 2026, 
https://encord.com/blog/vision-based-localization-a-guide-to-vbl-technique/
Drones For Distributed Swarm And Formation Flight - Fly4Future, erişim tarihi Mart 4, 
2026, https://fly4future.com/drones-for-distributed-swarm-and-formation-flight/
Advancement Challenges in UAV Swarm Formation Control: A Comprehensive Review, erişim 
tarihi Mart 4, 2026, https://www.mdpi.com/2504-446X/8/7/320
Drone Swarms as Networked Control Systems by Integration of Networking and Computing 
- PMC, erişim tarihi Mart 4, 2026, https://pmc.ncbi.nlm.nih.gov/articles/PMC8068910/
Formation Control Algorithms for Drone Swarms and The Single Point of Failure Crisis: 
A Review - ResearchGate, erişim tarihi Mart 4, 2026, 
https://www.researchgate.net/publication/382403635_Formation_Control_Algorithms_for_Drone_Swarms_and_The_Single_Point_of_Failure_Crisis_A_Review
Air–Ground Coordinated MEC: Joint Task, Time Allocation and Trajectory Design, erişim 
tarihi Mart 4, 2026, 
https://scholars.houstonmethodist.org/en/publications/airground-coordinated-mec-joint-task-time-allocation-and-trajecto/
Aerial and Ground Robot Collaboration for Autonomous Mapping in Search and Rescue 
Missions - MDPI, erişim tarihi Mart 4, 2026, https://www.mdpi.com/2504-446X/4/4/79
Stronger Together: Air-Ground Robotic Collaboration Using Semantics - YouTube, erişim 
tarihi Mart 4, 2026, https://www.youtube.com/watch?v=4e7eQaG9wB0
Collaborative Exploration with a Marsupial Ground-Aerial Robot Team through 
Task-Driven Map Compression - arXiv, erişim tarihi Mart 4, 2026, 
https://arxiv.org/html/2509.07655v1
3D Voxel-based Collaborative Path Planning for UAVs in Urban Emergency Response, 
erişim tarihi Mart 4, 2026, 
https://isprs-archives.copernicus.org/articles/XLVIII-G-2025/1573/2025/isprs-archives-XLVIII-G-2025-1573-2025.pdf
A Comprehensive Review of Path-Planning Algorithms for Multi-UAV Swarms - MDPI, 
erişim tarihi Mart 4, 2026, https://www.mdpi.com/2504-446X/10/1/11
4D BIM-Based Enriched Voxel Map for UAV Path Planning in Dynamic Construction 
Environments - MDPI, erişim tarihi Mart 4, 2026, 
https://www.mdpi.com/2079-8954/13/10/917
3D Trajectory Planning Method for UAVs Swarm in Building Emergencies - PMC, erişim 
tarihi Mart 4, 2026, https://pmc.ncbi.nlm.nih.gov/articles/PMC7038361/
Dynamic Pathfinding for a Swarm Intelligence Based UAV Control Model Using Particle 
Swarm Optimisation - Frontiers, erişim tarihi Mart 4, 2026, 
https://www.frontiersin.org/journals/applied-mathematics-and-statistics/articles/10.3389/fams.2021.744955/full
AimLock: Leading the Charge in Semi-Autonomous Weapons Tech | Soldier Systems Daily, 
erişim tarihi Mart 4, 2026, 
https://soldiersystems.net/2024/05/22/aimlock-leading-the-charge-in-semi-autonomous-weapons-tech/
How AI Is Transforming Ammunition Market in North America - MarketsandMarkets, erişim 
tarihi Mart 4, 2026, 
https://www.marketsandmarkets.com/ResearchInsight/ai-impact-analysis-on-north-america-ammunition-market.asp
The Integration Of AI-Empowered Autonomous Weapon Systems In European Defence, erişim 
tarihi Mart 4, 2026, https://tdhj.org/blog/post/ai-autonomous-weapons-europe/
Smart Logistics: Navigating the AI Frontier in Sustainment Operations | Article - 
Army.mil, erişim tarihi Mart 4, 2026, 
https://www.army.mil/article/280377/smart_logistics_navigating_the_ai_frontier_in_sustainment_operations
Magic Bullets: The Future of Artificial Intelligence in Weapons Systems | Article - 
U.S. Army, erişim tarihi Mart 4, 2026, 
https://www.army.mil/article/223026/magic_bullets_the_future_of_artificial_intelligence_in_weapons_systems
New UAV Navigation Estimator version: now with OLHIC, erişim tarihi Mart 4, 2026, 
https://www.uavnavigation.com/company/blog/new-uav-navigation-estimator-version-now-olhic
Magnetic Declination and Navigation Database Update — Flaps 2 Approach - Boeing 737 
Simulator project, erişim tarihi Mart 4, 2026, 
https://www.flaps2approach.com/journal/2017/1/20/magnetic-declination-and-navigation-database-update.html
Best way to correct compass deviation?, erişim tarihi Mart 4, 2026, 
https://discuss.px4.io/t/best-way-to-correct-compass-deviation/3909
JamShield: A Machine Learning Detection System for Over-the-Air Jamming Attacks - 
arXiv, erişim tarihi Mart 4, 2026, https://arxiv.org/html/2507.11483v1
An SDR-Based Real-Time Testbed for GNSS Adaptive Array Anti-Jamming Algorithms 
Accelerated by GPU - MDPI, erişim tarihi Mart 4, 2026, 
https://www.mdpi.com/1424-8220/16/3/356
A real-time capable software-defined receiver using GPU for adaptive anti-jam GPS 
sensors, erişim tarihi Mart 4, 2026, https://pubmed.ncbi.nlm.nih.gov/22164116/
Leveraging Advanced ML to Detect and Mitigate Spoofing and Jamming - Inside GNSS, 
erişim tarihi Mart 4, 2026, 
https://insidegnss.com/leveraging-advanced-ml-to-detect-and-mitigate-spoofing-and-jamming/
(PDF) Development of a Receiver-Agnostic and Real-Time Jamming and Spoofing Detection 
and Mitigation System - ResearchGate, erişim tarihi Mart 4, 2026, 
https://www.researchgate.net/publication/396108862_Development_of_a_Receiver-Agnostic_and_Real-Time_Jamming_and_Spoofing_Detection_and_Mitigation_System
Software-Defined Radio for Engineers - ARTECH HOUSE USA, erişim tarihi Mart 4, 2026, 
https://us.artechhouse.com/Software-Defined-Radio-for-Engineers-P1951.aspx
The Next-Gen Software-Defined Radio (SDR) Transceiver Delivers Big Advances in 
Frequency Hopping (FH) | Analog Devices, erişim tarihi Mart 4, 2026, 
https://www.analog.com/en/resources/analog-dialogue/articles/the-next-gen-software-defined-radio-sdr-transceiver-delivering-big-advances-in-frequency-hopping-fh.html
Introduction to Software Defined Radios (SDR) - everything RF, erişim tarihi Mart 4, 
2026, 
https://cdn.everythingrf.com/wpfiles/introduction_to_software_defined_radios_sdr__639002511891467544.pdf
Software Defined Radio (SDR) and Decoding On-off Keying (OOK) - Compass Security 
Blog, erişim tarihi Mart 4, 2026, 
https://blog.compass-security.com/2016/09/software-defied-radio-sdr-and-decoding-on-off-keying-ook/
Enhanced Spectrum Monitoring Capabilities Based on Software-Defined Radio Systems: 
Validation of a Real Time Frequency Hopping Signal Detection | CoLab, erişim tarihi 
Mart 4, 2026, https://colab.ws/articles/10.2478%2Fkbo-2024-0099
citadel_pqcrypto - Rust - Docs.rs, erişim tarihi Mart 4, 2026, 
https://docs.rs/citadel_pqcrypto
open-quantum-safe/liboqs: C library for prototyping and experimenting with 
quantum-resistant cryptography - GitHub, erişim tarihi Mart 4, 2026, 
https://github.com/open-quantum-safe/liboqs
Secure Software Environment - PQShield, erişim tarihi Mart 4, 2026, 
https://pqshield.com/use_cases/application-integrity-agility/
Post Quantum Cryptography with rust - The Rust Programming Language Forum, erişim 
tarihi Mart 4, 2026, 
https://users.rust-lang.org/t/post-quantum-cryptography-with-rust/94001
Low Probability of Intercept Radar Signal Recognition Based on Semi-Supervised 
Support Vector Machine - MDPI, erişim tarihi Mart 4, 2026, 
https://www.mdpi.com/2079-9292/13/16/3248
Low-Probability-of-Intercept Cellular and Wi-Fi Communications - Huacheng Zeng, 
erişim tarihi Mart 4, 2026, https://inss.egr.msu.edu/lpi.html
Low-probability of Intercept/Detect (LPI/LPD) Secure Communications Using Antenna 
Arrays Employing Rapid Sidelobe Time Modulation - arXiv, erişim tarihi Mart 4, 2026, 
https://arxiv.org/html/2406.11229v1
Advanced Low Probability of Intercept/Low Probability of Detection Radar (LPI/LPD) 
Techniques Using Artificial Intelligence Driven Methods - Navy SBIR/STTR, erişim 
tarihi Mart 4, 2026, https://www.navysbir.com/n21_2/N212-114.htm
Phase I/II SBIR NAVAIR - Low Probability of Detection/Low Probability of Interception 
(Smart Beam-QRC) | Forward Edge Ai, Inc. Help Center, erişim tarihi Mart 4, 2026, 
https://support.forwardedge.ai/en/articles/8781279-phase-i-ii-sbir-navair-low-probability-of-detection-low-probability-of-interception-smart-beam-qrc
AI-Enabled Cyberspace Operations - The Defence Horizon Journal, erişim tarihi Mart 4, 
2026, https://tdhj.org/blog/post/ai-cyberspace-operations/
A Quantitative Analysis of Offensive Cyber Operation (OCO) Automation Tools - NSF 
Public Access Repository, erişim tarihi Mart 4, 2026, 
https://par.nsf.gov/servlets/purl/10430173
AI Reinvents Complex Cyber Attack Replication for Critical Infrastructure Protection, 
erişim tarihi Mart 4, 2026, 
https://www.afcea.org/signal-media/cyber-edge/ai-reinvents-complex-cyber-attack-replication-critical-infrastructure
AI Automation in Cyber Security. In the modern landscape of offensive… | by Dhiraj 
Ambigapathi | Feb, 2026 | Medium, erişim tarihi Mart 4, 2026, 
https://medium.com/@Darkhue/ai-automation-in-cyber-security-7efa302f0d66
Countering Autonomous Cyber Threats - arXiv, erişim tarihi Mart 4, 2026, 
https://arxiv.org/html/2410.18312v1
Radar Cross Section Reduction Techniques for Drones - GreyB, erişim tarihi Mart 4, 
2026, https://xray.greyb.com/drones/radar-cross-section-reduction-stealth
Radar Target Radar Cross-Section Measurement Based on Enhanced Imaging and Scattering 
Center Extraction - MDPI, erişim tarihi Mart 4, 2026, 
https://www.mdpi.com/1424-8220/24/19/6315
RCS and Scattering Simulation for Radar Systems - Altair, erişim tarihi Mart 4, 2026, 
https://altair.com/resource/rcs-and-scattering-simulation-for-radar-systems
Radar Cross section modeling - DCS: F-35A - ED Forums, erişim tarihi Mart 4, 2026, 
https://forum.dcs.world/topic/367866-radar-cross-section-modeling/
(PDF) J9: Radar Cross Section Analysis of Stealth Fighter Design: Key Factors and 
Limitation of Simulation - ResearchGate, erişim tarihi Mart 4, 2026, 
https://www.researchgate.net/publication/312373207_J9_Radar_Cross_Section_Analysis_of_Stealth_Fighter_Design_Key_Factors_and_Limitation_of_Simulation

