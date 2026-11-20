// 一键成城 · 立体沙盘（5.0）
// 页面层三维搭建：只读核心导出的场景 JSON（几何＋配色），不参与生成。
// 依赖 Three.js（three.min.js + OrbitControls，本地打包，离线可用）。
// 坐标换算：城图 (x, y) → 沙盘 (x-640, 高, y-480)；形状先取 (x, -y) 再绕 X 轴 -90° 铺平。
(function (global) {
  'use strict';

  var HEIGHTS = { '民居': 7, '市集': 9, '衙署': 15, '寺庙': 13, '学宫': 11, '园林': 3, '驿站': 9, '水市': 7, '码头': 4 };

  function shapeFrom(THREE, poly) {
    var s = new THREE.Shape();
    poly.forEach(function (p, i) {
      if (i === 0) s.moveTo(p.x, -p.y); else s.lineTo(p.x, -p.y);
    });
    s.closePath();
    return s;
  }

  // ribbon 把一条折线铺成带状平面（河流、街道、护城河用）。
  function ribbon(THREE, pts, width, color, y, closed) {
    var P = closed ? pts.concat([pts[0]]) : pts;
    var pos = [];
    for (var i = 0; i < P.length; i++) {
      var p = P[i];
      var pPrev = P[Math.max(0, i - 1)], pNext = P[Math.min(P.length - 1, i + 1)];
      var dx = pNext.x - pPrev.x, dy = pNext.y - pPrev.y;
      var l = Math.hypot(dx, dy) || 1; dx /= l; dy /= l;
      var nx = -dy * width / 2, ny = dx * width / 2;
      pos.push(p.x - 640 + nx, 0, p.y - 480 + ny, p.x - 640 - nx, 0, p.y - 480 - ny);
    }
    var idx = [];
    for (var j = 0; j + 1 < P.length; j++) {
      var a = j * 2;
      idx.push(a, a + 1, a + 2, a + 1, a + 3, a + 2);
    }
    var g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
    g.setIndex(idx);
    g.computeVertexNormals();
    var m = new THREE.Mesh(g, new THREE.MeshLambertMaterial({ color: color, side: THREE.DoubleSide }));
    m.position.y = y;
    return m;
  }

  function buildCityGroup(THREE, S) {
    var C = S.colors;
    function mat(c) { return new THREE.MeshLambertMaterial({ color: c }); }
    var g = new THREE.Group();

    // 纸面大地
    var base = new THREE.Mesh(new THREE.PlaneGeometry(1500, 1150), mat(C.paper));
    base.rotation.x = -Math.PI / 2;
    base.position.y = -0.3;
    base.receiveShadow = true;
    g.add(base);

    // 城外田野
    (S.fields || []).forEach(function (f) {
      var m = new THREE.Mesh(new THREE.ShapeGeometry(shapeFrom(THREE, f.Poly)), mat(f.Tone === 1 ? C.fieldB : C.fieldA));
      m.rotation.x = -Math.PI / 2;
      m.position.set(-640, 0.06, -480);
      g.add(m);
    });

    // 城内地面
    var ground = new THREE.Mesh(new THREE.ShapeGeometry(shapeFrom(THREE, S.boundary)), mat(C.ground));
    ground.rotation.x = -Math.PI / 2;
    ground.position.set(-640, 0.12, -480);
    ground.receiveShadow = true;
    g.add(ground);

    // 水系：城南河、运河、护城河
    if (S.river && S.river.length > 1) g.add(ribbon(THREE, S.river, 46, C.water, 0.25, false));
    (S.canals || []).forEach(function (cn) { g.add(ribbon(THREE, cn, 15, C.water, 0.55, false)); });
    var moat = S.boundary.map(function (p) {
      return { x: 640 + (p.x - 640) * 1.14, y: 480 + (p.y - 480) * 1.14 };
    });
    g.add(ribbon(THREE, moat, 15, C.water, 0.25, true));

    // 街道：环街与主街
    if (S.ring && S.ring.length > 3) g.add(ribbon(THREE, S.ring, 8, C.road, 0.4, true));
    (S.roads || []).forEach(function (r) { g.add(ribbon(THREE, r, 11, C.road, 0.4, false)); });

    // 坊块（按用途定高）＋地标大屋顶
    (S.wards || []).forEach(function (w) {
      var h = HEIGHTS[w.kind] || 7;
      var geo = new THREE.ExtrudeGeometry(shapeFrom(THREE, w.poly), { depth: h, bevelEnabled: false });
      var m = new THREE.Mesh(geo, mat(w.fill));
      m.rotation.x = -Math.PI / 2;
      m.position.set(-640, 0, -480);
      m.castShadow = true;
      m.receiveShadow = true;
      g.add(m);
      if (w.kind === '衙署' || w.kind === '寺庙' || w.kind === '学宫') {
        var cx = 0, cy = 0, minx = 1e9, maxx = -1e9, miny = 1e9, maxy = -1e9;
        w.poly.forEach(function (p) {
          cx += p.x; cy += p.y;
          minx = Math.min(minx, p.x); maxx = Math.max(maxx, p.x);
          miny = Math.min(miny, p.y); maxy = Math.max(maxy, p.y);
        });
        cx /= w.poly.length; cy /= w.poly.length;
        var rw = Math.max(maxx - minx, maxy - miny) * 0.34;
        var roof = new THREE.Mesh(new THREE.ConeGeometry(rw, 10, 4), mat(C.roofBig));
        roof.position.set(cx - 640, h + 5, cy - 480);
        roof.rotation.y = Math.PI / 4;
        roof.scale.z = Math.max(0.5, Math.min(1.6, (maxy - miny) / (maxx - minx || 1)));
        roof.castShadow = true;
        g.add(roof);
      }
    });

    // 屋舍：小盒＋四坡小顶
    (S.buildings || []).forEach(function (bd) {
      var box = new THREE.Mesh(new THREE.BoxGeometry(bd.HW * 2, 5, bd.HH * 2), mat(C.building));
      box.position.set(bd.CX - 640, 2.5, bd.CY - 480);
      box.rotation.y = -bd.Ang;
      box.castShadow = true;
      g.add(box);
      var roof = new THREE.Mesh(new THREE.ConeGeometry(bd.HW * 1.45, 3.2, 4), mat(C.roof));
      roof.position.set(bd.CX - 640, 6.6, bd.CY - 480);
      roof.rotation.y = Math.PI / 4 - bd.Ang;
      roof.scale.z = bd.HH / bd.HW;
      g.add(roof);
    });

    // 城墙＋门楼
    (S.wallEdges || []).forEach(function (e) {
      var a = e[0], b = e[1];
      var len = Math.hypot(b.x - a.x, b.y - a.y);
      var wall = new THREE.Mesh(new THREE.BoxGeometry(len + 2, 15, 5.5), mat(C.wall));
      wall.position.set((a.x + b.x) / 2 - 640, 7.5, (a.y + b.y) / 2 - 480);
      wall.rotation.y = -Math.atan2(b.y - a.y, b.x - a.x);
      wall.castShadow = true;
      wall.receiveShadow = true;
      g.add(wall);
    });
    (S.gates || []).forEach(function (gt) {
      var tower = new THREE.Mesh(new THREE.BoxGeometry(17, 23, 17), mat(C.wallTop));
      tower.position.set(gt.mid.x - 640, 11.5, gt.mid.y - 480);
      tower.castShadow = true;
      g.add(tower);
      var roof = new THREE.Mesh(new THREE.ConeGeometry(15, 7, 4), mat(C.roofBig));
      roof.position.set(gt.mid.x - 640, 26.5, gt.mid.y - 480);
      roof.rotation.y = Math.PI / 4;
      g.add(roof);
    });

    return g;
  }

  // createViewer 在 container 里建渲染器；getScene() 返回 Promise<场景 JSON>。
  function createViewer(THREE, container, getScene) {
    var renderer = new THREE.WebGLRenderer({ antialias: true });
    renderer.setPixelRatio(Math.min(global.devicePixelRatio || 1, 2));
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    container.appendChild(renderer.domElement);

    var scene = new THREE.Scene();
    var camera = new THREE.PerspectiveCamera(42, 16 / 10, 1, 5000);
    camera.position.set(120, 560, 780);

    var controls = new THREE.OrbitControls(camera, renderer.domElement);
    controls.enableDamping = true;
    controls.dampingFactor = 0.06;
    controls.maxPolarAngle = 1.35;
    controls.minDistance = 260;
    controls.maxDistance = 1900;
    controls.autoRotate = true;
    controls.autoRotateSpeed = 0.6;
    controls.addEventListener('start', function () { controls.autoRotate = false; });

    var hemi = new THREE.HemisphereLight(0xfff4e0, 0x8a7a5a, 0.95);
    scene.add(hemi);
    var sun = new THREE.DirectionalLight(0xffe8c0, 0.85);
    sun.position.set(420, 620, 240);
    sun.castShadow = true;
    sun.shadow.mapSize.set(1024, 1024);
    sun.shadow.camera.left = -580; sun.shadow.camera.right = 580;
    sun.shadow.camera.top = 580; sun.shadow.camera.bottom = -580;
    scene.add(sun);

    var group = null;
    var visible = false;

    function resize() {
      var w = container.clientWidth || 800;
      var h = Math.round(w * 0.62);
      renderer.setSize(w, h);
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
    }
    if (global.ResizeObserver) new ResizeObserver(resize).observe(container);
    resize();

    (function loop() {
      requestAnimationFrame(loop);
      if (visible) {
        controls.update();
        renderer.render(scene, camera);
      }
    })();

    return {
      setVisible: function (v) { visible = v; if (v) resize(); },
      refresh: function () {
        return getScene().then(function (S) {
          if (S.region) return 'region';
          scene.background = new THREE.Color(S.colors.paper);
          scene.fog = new THREE.Fog(S.colors.paper, 1500, 2800);
          if (S.theme === 'night') { hemi.intensity = 0.5; sun.intensity = 0.45; }
          else { hemi.intensity = 0.95; sun.intensity = 0.85; }
          if (group) scene.remove(group);
          group = buildCityGroup(THREE, S);
          scene.add(group);
          resize();
          return 'ok';
        });
      }
    };
  }

  global.YC3D = { buildCityGroup: buildCityGroup, createViewer: createViewer, HEIGHTS: HEIGHTS };
})(typeof window !== 'undefined' ? window : globalThis);
