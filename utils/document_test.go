/*
  Copyright 2023 NanaFS Authors.

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

package utils

import "testing"

func TestContentTrim(t *testing.T) {
	type args struct {
		contentType string
		content     string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				contentType: "html",
				content:     "<h1>聊聊信息与数字技术，对于现代经济的基础意义</h1>\n<div id=\"readability-page-1\" class=\"page\">\n\n<p data-check-id=\"619724\">最近一个月来，互联网行业频频飞出“黑天鹅”，从阿里系产品连续出现问题，到滴滴因“底层软件故障”长时间崩溃。除了相关的技术漏洞，这些事故也折射出信息与数字技术已经深度渗透到我国经济的方方面面，从实体企业的数据安全到交易利益，从司机的收入到打工人的全勤奖，成为了“失去了才知道可贵”的基础设施、基础技术。今天我们就来聊聊信息与数字技术对于现代经济的基础意义。</p><p data-check-id=\"411857\">对西方经济学属于“庸俗经济学”的批判，其中一点就是粗暴地将所有经济活动用单一的货币量纲衡量，导致货币表象和实物工作量的差距。讨论数字技术是不是“硬技术”，也要从经济活动的本质出发：</p><p data-check-id=\"974800\">人类的经济活动本质上利用能源进行物质改造从而满足自身需求的过程，生产力就是判断这一过程能力与效率的指标。新石器时代革命是主动利用太阳能进行生化合成，第一次工业革命标志着首次摆脱人、畜或植物“现合成现应用”的能源利用模式限制，第二次工业革命则解决了高密度燃料和能源大规模传输问题——一句话，<strong>能够实实在在提高人类利用能源或改造物质能力的技术，就是基础技术、“硬技术”。</strong></p><p data-check-id=\"149667\"><img _src=\"https://img.huxiucdn.com/article/content/202312/05/071753333578.jpg?imageView2/2/w/1000/format/jpg/interlace/1/q/85\" data-w=\"1000\" data-h=\"653\" src=\"https://img.huxiucdn.com/article/content/202312/05/071753333578.jpg?imageView2/2/w/1000/format/jpg/interlace/1/q/85\" referrerpolicy=\"no-referrer\"/></p><p data-check-id=\"800903\">随着第一二次工业革命极大增加生产过程本身以及整个社会治理的复杂度，信息作为第三大生产力因素的关键性作用开始逐步凸显。信息减少了决策者面临的不确定性，让资源的投入更加精准高效，如果生产或社会治理系统超出了信息处理能力的约束，不能对复杂系统进行有效控制，就一定会步入僵化或混乱的结局。</p><p data-check-id=\"389264\">信息的收集、传递、计算处理和运用贯穿了人类的生产活动，绳结、竹简、纸张，算筹、算盘、机械计算器，这些都可以算作早期存储和计算处理信息的广义“信息技术”，跟随着社会经济信息处理量的膨胀而不断进步升级。电子科技让计算本身也拥有了工业化的规模和速度，数字化的信息存储运算成为了信息技术的最主要组成部分。</p><p data-check-id=\"254899\"><strong>以数字信息技术为标志的第三次工业革命虽然没有在能源或物质领域做出突破，但极大外推了这两大过程面临的算力约束，带来了社会生产力的实在进步，无愧于一次工业革命之名。</strong></p><p data-check-id=\"825108\">苏联的兴衰就是信息技术“硬度”的试金石。苏俄的数学超人们开发了一套经济控制论体系，用写满上下游产品关系的巨大表格网住所有社会产品的生产和调配，试图真真正正地把经济作为一项社会工程，用数学工具进行建设和维护。</p><p data-check-id=\"959047\">当苏联还是一个手扶木犁的农业国的时候，这张大表还能比较充分地代表相对简单的产业链和社会产品需求，搜集各级数据、解线性规划的时间成本还在良性范围内，实现了资源的高效配置，创造了快速工业化的奇迹。</p><p data-check-id=\"275261\"><img _src=\"https://img.huxiucdn.com/article/content/202312/05/071754856258.jpg?imageView2/2/w/1000/format/jpg/interlace/1/q/85\" data-w=\"1000\" data-h=\"951\" src=\"https://img.huxiucdn.com/article/content/202312/05/071754856258.jpg?imageView2/2/w/1000/format/jpg/interlace/1/q/85\" referrerpolicy=\"no-referrer\"/></p><p data-check-id=\"922423\">但等苏联完成了工业化，越来越多的产品加入平衡表，人民的需求更加多元化，<strong>经济的后续发展就已经超过了计委的模型与算力</strong>，计划本身的成本和错误率越来越高，计算人员越来越难以获取真实的生产数据，上报数据和修改计划的会越开越长，甚至做全年计划本身就能占到全年的1/12。</p><p data-check-id=\"538122\">苏联并非没有做过依靠数字技术提高算力的努力，先后尝试过АСПР和ОГАС等电子经济计算系统，但正如苏联计委人自嘲：“实现这玩意需要给每一头母牛装上传感器！”苏联的技术绝不足以完成如此大规模的数据采集和运算，本就存在各级官僚拍脑门、乱伸手问题自然更加猖獗，最终造成了苏联的经济失败。</p><p data-check-id=\"907337\">可见，苏联计划经济先胜后败的原因既非神神叨叨的人性欲望论，也非事后吹嘘的市场万能论，<strong>而是经济决策者的信息处理能力远远落后于它所控制系统的复杂程度。</strong>即使苏联倒下后人类的信息技术有了长足进步，但显然没有跟上社会经济复杂度的提高速度，把信息搜集、处理和决策的工作量分解到微观个体仍是符合当前算力基础的生产组织模式。</p><p data-check-id=\"314605\">每个企业就是一个计划经济体，各层管理人员根据自己掌握的信息下达指令完成各个商业环节，越是成功的企业越是优秀的计划者、系统控制者。现代企业的生产、管理和营销无不浸润在大数据之中，数字技术就是把习以为常的数据环境转化为生产力的“点金术”，通过改善分散的生产决策效率提高整个社会的生产效率。</p><p data-check-id=\"136946\">机器最先取代的是人类的体力劳动，一些重复性高但需要分析判断的工作却没有在第一二次工业革命中实现机器对人的替代。运用信息技术提高机器的算力、同时替代体力劳动和部分脑力劳动是深化工业革命的大方向，比如信件包裹的分拣以前需要靠人识读，现在的分拣中心则能自动化处理一年上千亿件的快递。<strong>可以说，数字技术把更多的“手工业”变成了“工业”，不断拓展着工业革命的外缘。</strong></p><p data-check-id=\"329182\">在计划编制、工况分析、故障监测、质量分析等等环节，一家智慧工厂每天能采集数百TB的数据，可见在以往的生产过程中存在着何等巨量的、未被得到利用的数据信息。建立起配套的数据分析系统，就能极大拓展生产流程中的可操作面，极大提高生产效率。</p><p data-check-id=\"589949\">而且生产主体分散决策并不意味着政府的算力需求下降，反而由于现代经济链条高度复杂和交叉，经济行为的外部性会沿着供应链被不断放大。政府更应当建立一套基于实物和服务生产的信息处理系统，充分反映流动的“链”的概念，好能够穿透扁平的货币核算的表象，对经济系统进行更有针对性和实时性的协调管理。</p><p data-check-id=\"274798\">所以，<strong>信息技术能让我们在可控核聚变等下一代能源物质利用技术突破到来前，</strong><strong>最大限度地挖掘生产力潜力、拓宽“工业化”的概念范围</strong>，是不折不扣的硬核技术、基底技术。相比之下，数字经济占GDP比重迅速提升只不过是技术重要性的结果体现而已。</p><p data-check-id=\"851397\"><img _src=\"https://img.huxiucdn.com/article/content/202312/05/071756776649.png?imageView2/2/w/1000/format/png/interlace/1/q/85\" data-w=\"277\" data-h=\"202\" src=\"https://img.huxiucdn.com/article/content/202312/05/071756776649.png?imageView2/2/w/1000/format/png/interlace/1/q/85\" referrerpolicy=\"no-referrer\"/></p><p data-check-id=\"337454\">生产力就是国力，围绕数字技术的大国竞争早已开始。美国的制裁往往被戏称为“高科技划重点”，对芯片等计算硬件的制裁已经得到了我们国家和公众的广泛重视和大力投入，华为“遥遥领先”标志着硬件技术反围剿已经开始见效，对于同样必须自主可控的计算软件，我们也应予以足够的重视。</p><p data-check-id=\"745838\">目前，软件、算法、控制系统等数字信息技术已经成为电力、交通、金融等“基础设施的基础设施”，若不能自主、依靠外部技术输入，大到国家的重大基础设施被留了“后门”，小到企业的敏感数据外泄，都将造成不可估量的损失。</p><p data-check-id=\"344140\">而在前沿竞争方面，无论是企业的竞争力，还是军队的战斗力，数字技术作为一种决定性的因素，<strong>也绝不能再抱着“造不如买”的侥幸心态，再次把卡脖子的主动权奉交别人手上。</strong></p><p data-check-id=\"193834\">美国人显然已经把数字技术列为了和硬件同等重要的战场，在计算软件方面的制裁施压并不逊于芯片。在大多数中美经贸谈判中，美国人必将开放云服务市场、取消外企数据本地存储作为谈判事项，持续敦促我国“公平”对待美国相关企业。美国却反手禁止用于训练AI的高端芯片对华出口，制裁中国的AI、超算企业，骚扰与中国合作的AI跨国公司，甚至matlab这种专业软件都能成为美国的制裁工具。</p><p data-check-id=\"557469\">特别是当下的世界，数字技术正在以超越想象的速度向前发展。今年10月16日，Gartner发布了2024年十大战略技术趋势，其中9个属于数字技术范畴，例如全民化的生成式AI、智能应用、行业云平台、AI增强开发等。福布斯也发布了AI领域五大趋势，涵盖生成式AI，AI道德，AI立法等。</p><p data-check-id=\"207974\"><img _src=\"https://img.huxiucdn.com/article/content/202312/05/071757429327.png?imageView2/2/w/1000/format/png/interlace/1/q/85\" data-w=\"817\" data-h=\"601\" src=\"https://img.huxiucdn.com/article/content/202312/05/071757429327.png?imageView2/2/w/1000/format/png/interlace/1/q/85\" referrerpolicy=\"no-referrer\"/></p><p data-check-id=\"773121\"><img _src=\"https://img.huxiucdn.com/article/content/202312/05/071758614203.png?imageView2/2/w/1000/format/png/interlace/1/q/85\" data-w=\"951\" data-h=\"475\" src=\"https://img.huxiucdn.com/article/content/202312/05/071758614203.png?imageView2/2/w/1000/format/png/interlace/1/q/85\" referrerpolicy=\"no-referrer\"/></p><p data-check-id=\"604233\">国内如腾讯研究院最近也发布了2024数字科技前沿应用趋势，其中涉及的领域，高性能计算、多模态智能体、AI+基因计算、星地直连通信等等，都很有可能成为下一个美国试图”卡脖子“的领域。</p><p data-check-id=\"241558\"><img _src=\"https://img.huxiucdn.com/article/content/202312/05/071759511547.png?imageView2/2/w/1000/format/png/interlace/1/q/85\" data-w=\"397\" data-h=\"832\" src=\"https://img.huxiucdn.com/article/content/202312/05/071759511547.png?imageView2/2/w/1000/format/png/interlace/1/q/85\" referrerpolicy=\"no-referrer\"/></p><p data-check-id=\"586004\">对此，我们已经采取了很多对应措施，从高铁信号控制系统自主化等重大项目，到坚持要求跨国公司数据本地存储，再到华为、腾讯、阿里、百度、讯飞等企业在商业云服务、AI辅助科研、智能协作机器人等领域加速研发突破。因为目前受制裁的数字技术还属于比较前沿高端的范畴，没有像芯片一样在民用领域造成广泛的切肤之痛，所以更应该提前布局、靠前发力，尽早开始缩小后发差距，乃至尽快争取引领地位。</p><p data-check-id=\"521650\">显然这一过程是任重道远的，令人吐槽无数的国产电脑操作系统，<strong>最近暴露出的大流量产品稳定性差等等问题都表明，我国数字技术的优质供给规模远远不足。</strong>在扩大供给的过程中，又有一些企业借着技术的幌子、赶着热点的风口大搞投机炒作，不仅侵占了本应用于发展技术的资金资源，还损害了实体企业和社会对信息技术的信任，以至于信息技术、数字行业本身属不属于实体经济都成为了一个争议颇大的话题。</p><p data-check-id=\"492036\">不光国家在反复强调数字经济属于实体、服务实体的导向，越来越多的理论也在突出数字经济的实体性技术性，比如最近在一个前沿科创论坛研讨上，经济学家樊纲就提出数字技术是硬科技、是需要自主可控的国家基础能力。除了概念上的厘清，更应该用市场激励的方式贯彻数字经济的实体定义，加强对投机炒作、无序扩张行为的监管，同时给予务实创新者相应的扶持激励，促使整个行业锚定在切切实实的技术创新和创造实体产值之上。</p><p data-check-id=\"994753\">过去十年，国内数字企业在toC端取得了巨大的成功，但也形成了技术服务商业的惯性思维。<strong>要想实现数字技术自主可控，行业就必须从商业模式驱动的技术创新转向技术创新驱动的商业模式，走出早些年在消费者端跑马圈地形成的舒适圈，到专业垂直领域“二次创业”。</strong></p><p data-check-id=\"702930\">就拿今年最火的ChatGPT举例，ChatGPT是一种通用大模型，通俗地说就是能和不具任何专业知识的用户“聊天”，不仅受众门槛低，而且调戏AI过程中的大量段子具有极强的传播力，一下子就引爆了舆论流量和投资流量。于是在国内企业推出大模型的热潮中，大量企业扎堆搞通用大模型，还闹出过“松鼠鳜鱼”的笑话。</p><p data-check-id=\"769275\"><img _src=\"https://img.huxiucdn.com/article/content/202312/05/071800040109.jpg?imageView2/2/w/1000/format/jpg/interlace/1/q/85\" data-w=\"762\" data-h=\"746\" src=\"https://img.huxiucdn.com/article/content/202312/05/071800040109.jpg?imageView2/2/w/1000/format/jpg/interlace/1/q/85\" referrerpolicy=\"no-referrer\"/></p><p data-check-id=\"363750\">但大模型要真正转化为生产力，就决不能止步于聊天、搜索、画画、做excel的通用场景，必须更进一步与各个产业的严肃场景深度结合，成为企业提升算力、增强决策能力的辅助工具，要把大模型的应用效果、深度作为“百模大战”成败的试金石。前文提到的智慧工厂、物流供应链等数据高度密集的地方应该成为大模型的主要战场，而对于医疗、科研、政府管理等因为需要高精度判断的、“手工业”程度仍然较高的行业，大模型则有可能成为它们“工业化”的起点。</p><p data-check-id=\"866440\">总的来看，有了芯片的前车之鉴，在这一轮数字信息技术革新的浪潮中，我们的政府、大企业及时意识到了自主可控的重要性，没有重蹈造不如买的覆辙，在很短时间内开始了追赶进程。<strong>但数字技术的进步迭代周期极快，我们的技术追赶仍然道阻且长。</strong></p><p data-check-id=\"878432\">更重要的是，在太多数字技术领域我们一直是“追赶者”，如何让我们成为数字技术的开创者引领者，深挖生产力的外沿，才是政府和数字行业的大企业和中小企业需要共同回答的历史课题。</p>\n\n</div>\n",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContentTrim(tt.args.contentType, tt.args.content); got != tt.want {
				t.Errorf("ContentTrim() = %v, want %v", got, tt.want)
			}
		})
	}
}
